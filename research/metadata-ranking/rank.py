"""Offline candidate ranker v1. No network access or production imports.
Weights fixed before running benchmark. Labels never enter ranking features.
"""
import math,re,unicodedata
WEIGHTS_VERSION='v1-2026-09-21'

def norm(s):
 return ' '.join(re.findall(r'[^\W_]+',unicodedata.normalize('NFKC',s).casefold()))

def isbn13(s):
 s=re.sub(r'[\s-]','',s).upper()
 if len(s)==10 and s[:9].isdigit() and (s[-1].isdigit() or s[-1]=='X'):
  if sum((10-i)*(10 if x=='X' else int(x)) for i,x in enumerate(s))%11:return None
  s='978'+s[:9]
  s+=str((-sum(int(x)*(1 if i%2==0 else 3) for i,x in enumerate(s)))%10)
 if len(s)!=13 or not s.isdigit() or not s.startswith(('978','979')):return None
 return s if sum(int(x)*(1 if i%2==0 else 3) for i,x in enumerate(s))%10==0 else None

def identity(d):return d.get('key',d.get('id','')).split('/')[-1].split(':')[-1]
def edition(d):return (d.get('editions',{}).get('docs') or [{}])[0]
def eligible(d):
 langs=edition(d).get('language',[])
 # Unknown edition language stays unknown; work languages never identify an edition.
 return not langs or 'eng' in langs

def score(query,d,qrank=None,trank=None,kind='book',cover=True):
 e=edition(d); q=norm(query)
 titles=[d.get('title',d.get('name','')),d.get('subtitle',''),e.get('title','')]
 tokens=set(norm(' '.join(titles)).split());qt=set(q.split())
 authors=d.get('author_name',[])
 exact=isbn13(query)
 matched=bool(exact and exact in {isbn13(i) for i in d.get('isbn',[])})
 parts={'provider_relevance':60/(1+.08*(qrank-1)) if qrank else 20/(1+.08*((trank or 41)-1)),
        'query_token_coverage':3*len(qt&tokens)/max(1,len(qt))}
 if kind=='author':
  parts['bibliography_evidence']=min(6,math.log2(1+d.get('work_count',0)))
  parts['exact_name']=3 if norm(d.get('name',''))==q else 0
 else:
  parts['author_present']=4 if authors else -10
  parts['identifier_present']=3 if any(isbn13(i) for i in d.get('isbn',[])) else 0
  parts['edition_language']=3 if 'eng' in e.get('language',[]) else 0
  parts['catalog_breadth']=min(6,math.log2(1+d.get('edition_count',0)))
  parts['cover_present']=1 if cover and (e.get('cover_i',0)>0 or d.get('cover_i',0)>0) else 0
  if d.get('edition_count',0)<=1 and not d.get('isbn') and not e.get('language'):
   parts['sparse_record']=-12
  text=norm(' '.join(titles))
  markers=('summary','study guide','workbook','coloring','colouring','trivia','quizzes','graphic novel','adaptation','soundtrack')
  if any(m in text and m not in q for m in markers):parts['unrequested_derivative']=-18
  if (d.get('title','').count(' / ')>=1 or any(m in text for m in ('boxed set','box set','collection set'))) and not any(m in q for m in ('collection','box','set','omnibus')):
   parts['unrequested_bundle']=-10
 return (int(matched),sum(parts.values())),{k:round(v,4) for k,v in parts.items()}

def rank(query,docs,qorder,torder=(),kind='book',cover=True):
 qr={identity(d):i+1 for i,d in enumerate(qorder)};tr={identity(d):i+1 for i,d in enumerate(torder)}
 rows=[]
 for d in docs:
  if kind!='author' and not eligible(d):continue
  s,parts=score(query,d,qr.get(identity(d)),tr.get(identity(d)),kind,cover)
  rows.append((s,identity(d),d,parts))
 rows.sort(key=lambda x:(-x[0][0],-x[0][1],x[1]))
 return rows
