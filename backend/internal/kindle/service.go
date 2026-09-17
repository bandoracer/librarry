package kindle

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var ErrUnavailable = errors.New("Kindle delivery requires database persistence")
var ErrConflict = errors.New("a send is already active or this request ID belongs to another document")

type Service struct {
	db        *sql.DB
	defaults  Settings
	ebookRoot string
	send      Sender
}

func New(db *sql.DB, defaults Settings, ebookRoot string) *Service {
	return &Service{db: db, defaults: defaults, ebookRoot: ebookRoot, send: sendSMTP}
}
func (s *Service) Available() bool { return s != nil && s.db != nil }
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	if !s.Available() {
		return Settings{}, ErrUnavailable
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `select settings from kindle_settings where singleton`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return s.defaults, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var v Settings
	err = json.Unmarshal(raw, &v)
	return v, err
}
func (s *Service) SaveSettings(ctx context.Context, v Settings, clearPassword bool) (Settings, error) {
	if !s.Available() {
		return Settings{}, ErrUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback()
	defaults, _ := json.Marshal(s.defaults)
	if _, err = tx.ExecContext(ctx, `insert into kindle_settings(singleton,settings) values(true,$1) on conflict do nothing`, defaults); err != nil {
		return Settings{}, err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, `select settings from kindle_settings where singleton for update`).Scan(&raw); err != nil {
		return Settings{}, err
	}
	var old Settings
	if err = json.Unmarshal(raw, &old); err != nil {
		return Settings{}, err
	}
	if clearPassword {
		v.Password = ""
	} else if v.Password == "" {
		v.Password = old.Password
	}
	v.PasswordConfigured = false
	v.Host = strings.TrimSpace(v.Host)
	v.From = strings.TrimSpace(v.From)
	v.Recipient = strings.TrimSpace(v.Recipient)
	v.Username = strings.TrimSpace(v.Username)
	if err = v.Validate(); err != nil {
		return Settings{}, err
	}
	raw, _ = json.Marshal(v)
	if _, err = tx.ExecContext(ctx, `update kindle_settings set settings=$1,updated_at=now() where singleton`, raw); err != nil {
		return Settings{}, err
	}
	return v, tx.Commit()
}

type Delivery struct {
	ID        string    `json:"id"`
	RequestID string    `json:"requestId"`
	WantedID  string    `json:"wantedId"`
	FileID    string    `json:"fileId"`
	Recipient string    `json:"recipient"`
	Sender    string    `json:"sender"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

const deliveryColumns = `id::text,request_id,wanted_id,file_id,recipient,sender,title,
case when state='sending' and created_at<now()-interval '2 minutes' then 'unknown' else state end,
case when state='sending' and created_at<now()-interval '2 minutes' then 'Submission interrupted; check Kindle before sending again' else message end,created_at`

type scanner interface{ Scan(...any) error }

func scanDelivery(row scanner) (Delivery, error) {
	var d Delivery
	err := row.Scan(&d.ID, &d.RequestID, &d.WantedID, &d.FileID, &d.Recipient, &d.Sender, &d.Title, &d.State, &d.Message, &d.CreatedAt)
	return d, err
}
func (s *Service) History(ctx context.Context, wantedID string) ([]Delivery, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	rows, err := s.db.QueryContext(ctx, `select `+deliveryColumns+` from kindle_deliveries where wanted_id=$1 order by created_at desc,id desc limit 50`, wantedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Delivery{}
	for rows.Next() {
		d, e := scanDelivery(rows)
		if e != nil {
			return nil, e
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

var requestPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,80}$`)

func (s *Service) Send(ctx context.Context, wantedID, fileID, requestID string) (Delivery, error) {
	if !s.Available() {
		return Delivery{}, ErrUnavailable
	}
	if !requestPattern.MatchString(requestID) {
		return Delivery{}, invalid("a 16–80 character request ID is required")
	}
	if (wantedID == "") != (fileID == "") {
		return Delivery{}, invalid("both book and file IDs are required")
	}
	// Replays return the recorded result, including after settings or files change.
	old, err := scanDelivery(s.db.QueryRowContext(ctx, `select `+deliveryColumns+` from kindle_deliveries where request_id=$1`, requestID))
	if err == nil {
		if old.WantedID != wantedID || old.FileID != fileID {
			return Delivery{}, ErrConflict
		}
		return old, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Delivery{}, err
	}
	cfg, err := s.Settings(ctx)
	if err != nil {
		return Delivery{}, err
	}
	if !cfg.Enabled {
		return Delivery{}, invalid("enable Kindle delivery in Settings first")
	}
	if err = cfg.Validate(); err != nil {
		return Delivery{}, err
	}
	msg := Message{Title: "Librarry setup test", Filename: "librarry-test.txt", Data: []byte("Librarry can send documents to your Kindle.\n")}
	if fileID != "" {
		msg, err = s.document(ctx, wantedID, fileID)
		if err != nil {
			return Delivery{}, err
		}
	}
	// The timeout is shorter than the abandonment window. Uncertain attempts are
	// never automatically resumed after a crash or lost acknowledgement.
	if _, err = s.db.ExecContext(ctx, `update kindle_deliveries set state='unknown',message='Submission interrupted; check Kindle before sending again',updated_at=now() where state='sending' and created_at<now()-interval '2 minutes'`); err != nil {
		return Delivery{}, err
	}
	d, err := scanDelivery(s.db.QueryRowContext(ctx, `insert into kindle_deliveries(request_id,wanted_id,file_id,recipient,sender,title,state) values($1,$2,$3,$4,$5,$6,'sending') on conflict do nothing returning `+deliveryColumns, requestID, wantedID, fileID, cfg.Recipient, cfg.From, msg.Title))
	if errors.Is(err, sql.ErrNoRows) {
		d, err = scanDelivery(s.db.QueryRowContext(ctx, `select `+deliveryColumns+` from kindle_deliveries where request_id=$1`, requestID))
		if err != nil || d.WantedID != wantedID || d.FileID != fileID {
			return Delivery{}, ErrConflict
		}
		return d, nil
	}
	if err != nil {
		return Delivery{}, err
	}
	msg.ID = d.ID
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	d.State, d.Message = s.send(sendCtx, cfg, msg)
	// Completion survives a browser disconnect. A failed write leaves the durable
	// sending row to become unknown, and the same request cannot send again.
	saveCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	saved, err := scanDelivery(s.db.QueryRowContext(saveCtx, `update kindle_deliveries set state=$2,message=$3,updated_at=now() where id::text=$1 and state='sending' returning `+deliveryColumns, d.ID, d.State, d.Message))
	if errors.Is(err, sql.ErrNoRows) {
		saved, err = scanDelivery(s.db.QueryRowContext(saveCtx, `select `+deliveryColumns+` from kindle_deliveries where id::text=$1`, d.ID))
	}
	if err != nil {
		return Delivery{}, invalid("submission outcome could not be saved; check delivery history before sending again")
	}
	return saved, nil
}

func (s *Service) document(ctx context.Context, wantedID, fileID string) (Message, error) {
	var path, title, status, checksum string
	var recordedSize int64
	err := s.db.QueryRowContext(ctx, `select f.path,coalesce(nullif(w.title,''),f.title),f.import_status,coalesce(f.checksum,''),coalesce(f.size_bytes,0)
 from files f join file_wanted_links l on l.file_id=f.id join wanted_items w on w.id=l.wanted_item_id
 where f.id::text=$1 and w.id::text=$2 and w.status not in ('removed','ignored') and w.wanted_format='ebook' and f.media_format='ebook'
 and f.presence_state='present' and f.import_status in ('imported','available')
 and (select count(*) from file_wanted_links where file_id=f.id)=1
 and not exists(select 1 from import_operation_files pf join import_operations op on op.id=pf.operation_id where pf.destination_path=f.path and op.state<>'committed')`, fileID, wantedID).Scan(&path, &title, &status, &checksum, &recordedSize)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, invalid("select a present, completed EPUB or PDF linked only to this active book")
	}
	if err != nil {
		return Message{}, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".epub" && ext != ".pdf" {
		return Message{}, invalid("Kindle delivery supports EPUB and PDF; convert other formats first")
	}
	rows, err := s.db.QueryContext(ctx, `select path,use_calibre from root_folders where media_format='ebook' order by length(path) desc`)
	if err != nil {
		return Message{}, err
	}
	type root struct {
		path    string
		calibre bool
	}
	roots := []root{}
	for rows.Next() {
		var r root
		if err = rows.Scan(&r.path, &r.calibre); err != nil {
			rows.Close()
			return Message{}, err
		}
		roots = append(roots, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Message{}, err
	}
	if len(roots) == 0 && s.ebookRoot != "" {
		roots = append(roots, root{path: s.ebookRoot})
	}
	for _, r := range roots {
		rel, e := filepath.Rel(r.path, path)
		if e != nil || !filepath.IsLocal(rel) {
			continue
		}
		if r.calibre {
			return Message{}, invalid("Calibre-managed files must be exported to a native library root before sending")
		}
		data, e := readDocument(r.path, rel, ext, recordedSize, checksum)
		if e != nil {
			return Message{}, e
		}
		if len(title) > 300 {
			title = title[:300]
		}
		return Message{Title: title, Filename: "book" + ext, Data: data}, nil
	}
	return Message{}, invalid("file is outside the configured ebook library roots")
}

func readDocument(rootPath, relative, ext string, size int64, checksum string) ([]byte, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, invalid("ebook root is unavailable")
	}
	defer root.Close()
	f, err := root.OpenFile(relative, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, invalid("book file is unavailable or escapes its library root")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, invalid("book must be a regular file")
	}
	if info.Size() == 0 || info.Size() > MaxFileBytes {
		return nil, invalid("book must be nonempty and no larger than 25 MiB")
	}
	if size > 0 && size != info.Size() {
		return nil, invalid("book size changed; rescan before sending")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil || len(data) > MaxFileBytes || int64(len(data)) != info.Size() {
		return nil, invalid("book changed or could not be read")
	}
	after, err := f.Stat()
	if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, invalid("book changed while being read")
	}
	if checksum != "" {
		hash := sha256.Sum256(data)
		if !strings.EqualFold(checksum, hex.EncodeToString(hash[:])) {
			return nil, invalid("book checksum changed; rescan before sending")
		}
	}
	if ext == ".pdf" && !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, invalid("file is not a PDF document")
	}
	if ext == ".epub" {
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil {
			return nil, invalid("file is not an EPUB archive")
		}
		valid := false
		for _, entry := range z.File {
			if entry.Name == "mimetype" {
				r, e := entry.Open()
				if e != nil {
					break
				}
				m, _ := io.ReadAll(io.LimitReader(r, 100))
				r.Close()
				valid = string(m) == "application/epub+zip"
				break
			}
		}
		if !valid {
			return nil, invalid("EPUB mimetype is missing or invalid")
		}
	}
	return data, nil
}
