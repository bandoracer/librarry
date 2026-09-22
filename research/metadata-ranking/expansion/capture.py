#!/usr/bin/env python3
"""A new diagnostic cohort; intended works written before observing provider ranks."""
import datetime, json, pathlib, time, urllib.request, urllib.parse
ROOT=pathlib.Path(__file__).resolve().parent
CASES=[
('braiding','Braiding Sweetgrass','Robin Wall Kimmerer','English'),
('dawn','The Dawn of Everything','David Graeber; David Wengrow','English'),
('entangled','Entangled Life','Merlin Sheldrake','English'),
('why','The Book of Why','Judea Pearl; Dana Mackenzie','English'),
('godel','Gödel Escher Bach','Douglas Hofstadter','English'),
('c-language','The C Programming Language','Brian Kernighan; Dennis Ritchie','English'),
('data','Designing Data-Intensive Applications','Martin Kleppmann','English'),
('salt','Salt Fat Acid Heat','Samin Nosrat','English'),
('food','The Food Lab','J. Kenji López-Alt','English'),
('body','The Body Keeps the Score','Bessel van der Kolk','English'),
('war','The Art of War','Sun Tzu','English'),
('meditations','Meditations','Marcus Aurelius','English'),
('room',"A Room of One's Own",'Virginia Woolf','English'),
('frankenstein','Frankenstein','Mary Shelley','English'),
('beloved','Beloved','Toni Morrison','English'),
('kindred','Kindred','Octavia Butler','English'),
('piranesi','Piranesi','Susanna Clarke','English'),
('memory','The Memory Police','Yoko Ogawa','English'),
('convenience','Convenience Store Woman','Sayaka Murata','English'),
('employees','The Employees','Olga Ravn','English'),
('plow','Drive Your Plow Over the Bones of the Dead','Olga Tokarczuk','English'),
('solitude','One Hundred Years of Solitude','Gabriel García Márquez','English'),
('soledad','Cien años de soledad','Gabriel García Márquez','Spanish'),
('etranger',"L'étranger",'Albert Camus','French'),
('tunel','El túnel','Ernesto Sábato','Spanish'),
('bjorn','Björnstad','Fredrik Backman','Swedish'),
('summer','The Summer Book','Tove Jansson','English'),
('psalm','A Psalm for the Wild-Built','Becky Chambers','English'),
('fifth','The Fifth Season','N. K. Jemisin','English'),
('seven','A Brief History of Seven Killings','Marlon James','English'),
('catalonia','Homage to Catalonia','George Orwell','English'),
('genji','The Tale of Genji','Murasaki Shikibu','English'),
('parable','Parable of the Sower by Octavia E Butler','Octavia E. Butler','English'),
('devotions','Devotions Mary Oliver','Mary Oliver','English'),
]
FIELDS='key,title,subtitle,author_name,author_key,first_publish_year,language,cover_i,editions,editions.key,editions.title,editions.language,editions.isbn,editions.cover_i,editions.publisher,editions.publish_date'
def query_text(value):
 for ch in '\\:"+-()[]{}^~*?!|&': value=value.replace(ch,' ')
 return value.lower()
def main():
 cases=[dict(id=i,query=q,intendedAuthor=a,language=l) for i,q,a,l in CASES]
 casepath=ROOT/'cases.json'
 if not casepath.exists():casepath.write_text(json.dumps(cases,ensure_ascii=False,indent=2)+'\n')
 langs={'English':'en','French':'fr','Spanish':'es','Swedish':'sv'}
 last=0
 for c in cases:
  p=ROOT/'snapshots'/(c['id']+'.json')
  if p.exists():continue
  time.sleep(max(0,1.1-(time.monotonic()-last)));last=time.monotonic()
  url='https://openlibrary.org/search.json?'+urllib.parse.urlencode(dict(q=query_text(c['query']),lang=langs[c['language']],limit=25,fields=FIELDS))
  record=dict(url=url,retrievedAt=datetime.datetime.now(datetime.timezone.utc).isoformat())
  try:
   with urllib.request.urlopen(urllib.request.Request(url,headers={'User-Agent':'LibrarryRankingResearch/0.1 (https://github.com/bandoracer/librarry)'}),timeout=15) as r:record.update(status=r.status,data=json.load(r))
  except Exception as e:record['error']=str(e)
  record['elapsedSeconds']=round(time.monotonic()-last,3)
  p.write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n')
  print(c['id'],record.get('status',record.get('error')),flush=True)
if __name__=='__main__':main()
