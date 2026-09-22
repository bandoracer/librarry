#!/usr/bin/env python3
"""Read-only, rate-limited research capture. Existing snapshots are not overwritten."""
import datetime,json,pathlib,time,urllib.request,urllib.parse,urllib.error
ROOT=pathlib.Path(__file__).resolve().parent
CASES=[
 ('percy','percy jackson','book','development'),
 ('lightning','the lightning thief','book','development'),
 ('hail-mary','project hail mary','book','development'),
 ('dune','dune','book','development'),
 ('earthsea','earthsea','book','development'),
 ('riordan','rick riordan','author','development'),
 ('guide','percy jackson the ultimate guide','book','development'),
 ('hobbit','the hobbit','book','validation'),
 ('pride','pride and prejudice','book','validation'),
 ('murderbot','murderbot','book','validation'),
 ('left-hand','the left hand of darkness','book','validation'),
 ('isbn','9781423103349','book','validation'),
 ('martian','the martian','book','blind'),
 ('watership','watership down','book','blind'),
 ('pern','dragonriders of pern','book','blind'),
 ('wizard-author','a wizard of earthsea by ursula k le guin','book','blind'),
]
FIELDS='key,title,subtitle,author_name,author_key,first_publish_year,isbn,edition_key,language,cover_i,edition_count,ratings_count,want_to_read_count,readinglog_count,editions,editions.key,editions.title,editions.language,editions.isbn,editions.cover_i,editions.publisher,editions.publish_date'
last=0

def fetch(name,url):
 global last
 p=ROOT/'snapshots'/f'{name}.json'
 if p.exists():return
 time.sleep(max(0,1.1-(time.monotonic()-last)))
 started=time.monotonic();last=started
 record={'url':url,'retrievedAt':datetime.datetime.now(datetime.timezone.utc).isoformat()}
 try:
  req=urllib.request.Request(url,headers={'User-Agent':'LibrarryRankingResearch/0.1 (https://github.com/bandoracer/librarry)'})
  with urllib.request.urlopen(req,timeout=20) as r:record.update(status=r.status,data=json.load(r))
 except Exception as e:record.update(error=str(e))
 record['elapsedSeconds']=round(time.monotonic()-started,3)
 p.write_text(json.dumps(record,indent=2)+'\n')
 print(name,record.get('status',record.get('error')),flush=True)

if __name__=='__main__':
 (ROOT/'cases.json').write_text(json.dumps([dict(id=i,query=q,type=t,split=s) for i,q,t,s in CASES],indent=2)+'\n')
 for i,q,t,s in CASES:
  fetch(i+'-live','http://192.168.1.221:30200/api/v1/search?'+urllib.parse.urlencode(dict(query=q,type=t,format='any',language='English')))
  if t=='author':
   fetch(i+'-direct','https://openlibrary.org/search/authors.json?'+urllib.parse.urlencode(dict(q=q,limit=40)))
  elif i=='isbn':
   fetch(i+'-direct','https://openlibrary.org/search.json?'+urllib.parse.urlencode(dict(isbn=q,fields=FIELDS,limit=40,lang='en')))
  else:
   for mode in ('title','q'):
    fetch(i+'-'+mode,'https://openlibrary.org/search.json?'+urllib.parse.urlencode({mode:q,'fields':FIELDS,'limit':40,'lang':'en'}))
 fetch('google-isbn','https://www.googleapis.com/books/v1/volumes?q=isbn:9781423103349&maxResults=5')
