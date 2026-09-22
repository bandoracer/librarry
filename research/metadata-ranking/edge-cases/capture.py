#!/usr/bin/env python3
"""Predeclared edge cases; fetch once and retain unsuccessful observations too."""
import datetime, json, pathlib, time, urllib.request, urllib.parse
ROOT=pathlib.Path(__file__).resolve().parent
CASES=[
 dict(id='employees-author',query='The Employees by Olga Ravn',target='OL25803688W'),
 dict(id='employees-suffix',query='The Employees Olga Ravn',target='OL25803688W'),
 dict(id='percy-typo',query='percy jakson',target='OL492658W'),
 dict(id='hail-typo',query='project hail maryy',target='OL21745884W'),
 dict(id='hobbit-typo',query='the hobbitt',target='OL27482W'),
 dict(id='punctuation',query='Project: Hail Mary!',target='OL21745884W'),
 dict(id='nonexistent',query='zxqvplm nonexistent book 782643',empty=True),
 dict(id='nonexistent-author',query='zxqvplm nonexistent book by Qzx Vplm',empty=True),
 dict(id='employees-audio',query='The Employees by Olga Ravn',target='OL25803688W',format='audiobook'),
 dict(id='hail-audio',query='Project Hail Mary',target='OL21745884W',format='audiobook'),
]
FIELDS='key,title,subtitle,author_name,author_key,first_publish_year,language,cover_i,editions,editions.key,editions.title,editions.language,editions.isbn,editions.cover_i,editions.publisher,editions.publish_date'
def literal(v):
 for c in '\\:"+-()[]{}^~*?!|&': v=v.replace(c,' ')
 return v.lower()
def fetch(name,values):
 p=ROOT/'snapshots'/(name+'.json')
 if p.exists():return
 url='https://openlibrary.org/search.json?'+urllib.parse.urlencode(dict(**values,lang='en',limit=25,fields=FIELDS))
 rec=dict(url=url,retrievedAt=datetime.datetime.now(datetime.timezone.utc).isoformat())
 try:
  with urllib.request.urlopen(urllib.request.Request(url,headers={'User-Agent':'LibrarryRankingResearch/0.1 (https://github.com/bandoracer/librarry)'}),timeout=20) as r:rec.update(status=r.status,data=json.load(r))
 except Exception as e:rec['error']=str(e)
 p.write_text(json.dumps(rec,ensure_ascii=False,indent=2)+'\n');print(name,rec.get('status',rec.get('error')),flush=True);time.sleep(1.1)
if __name__=='__main__':
 p=ROOT/'cases.json'
 if not p.exists():p.write_text(json.dumps(CASES,indent=2)+'\n')
 for c in CASES:fetch(c['id'],dict(q=literal(c['query'])))
 for c in CASES:
  if ' by ' in c['query']:
   t,a=c['query'].rsplit(' by ',1);fetch(c['id']+'-rescue',dict(title=literal(t),author=literal(a)))

 for c in CASES:
  if " by " in c["query"]:
   t,a=c["query"].rsplit(" by ",1);fetch(c["id"]+"-author-rescue",dict(q=literal(t),author=literal(a)))
 fetch("parable-author-rescue",dict(q="parable of the sower",author="octavia e butler"))
