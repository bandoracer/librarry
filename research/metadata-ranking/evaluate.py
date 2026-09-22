#!/usr/bin/env python3
"""Compare frozen live results, same-pool reranking, retrieval, and candidate v1.
Metrics measure success against curated positive targets, NOT precision or nDCG.
Unjudged items are not asserted to be irrelevant. Network-free and reproducible.
"""
import json,pathlib,hashlib
from rank import rank,identity,eligible,edition,isbn13,WEIGHTS_VERSION
ROOT=pathlib.Path(__file__).resolve().parent

def load(name):return json.loads((ROOT/'snapshots'/f'{name}.json').read_text())['data']
def unique(ds):
 out={}
 for d in ds:out.setdefault(identity(d),d)
 return list(out.values())
def metrics(ids,label):
 pos=next((i+1 for i,k in enumerate(ids[:10]) if k in label['targets']),None)
 return dict(top1=int(bool(ids) and ids[0] in label['targets']),hit10=int(pos is not None),rr10=1/pos if pos else 0,targetRank=pos)

def legacy_score(query,d):
 from rank import norm
 q=norm(query);t=norm(d.get('title',d.get('name','')))
 qt=set(q.split());tt=set(t.split())
 score=.2+(.55 if q==t else .35 if q in t or t in q else .35*len(qt&tt)/max(1,len(qt|tt)))
 a=norm((d.get('author_name') or [''])[0])
 return score+(.18 if a and a in q else 0)

def evaluate():
 cases=json.loads((ROOT/'cases.json').read_text());labels=json.loads((ROOT/'labels.json').read_text());out=[]
 for c in cases:
  cid=c['id'];label=labels[cid];kind=c['type']
  qdocs=load(cid+('-direct' if cid in ('riordan','isbn') or kind=='author' else '-q'))['docs']
  tdocs=[] if cid in ('riordan','isbn') or kind=='author' else load(cid+'-title')['docs']
  pool=unique(qdocs+tdocs)
  live=load(cid+'-live')['results'];liveids=[identity(r['work']) for r in live]
  byid={identity(d):d for d in pool}
  # Enrich only surviving live candidates: this cannot repair missing candidates.
  livepool=[byid[k] for k in liveids if k in byid]
  rows=rank(c['query'],pool,qdocs,tdocs,kind)
  paths={'live':liveids,
         'same_pool_rerank':[x[1] for x in rank(c['query'],livepool,qdocs,tdocs,kind)],
         'title_language_fix':[identity(d) for d in sorted([d for d in (qdocs if cid == "isbn" or kind == "author" else tdocs)[:10] if kind=='author' or eligible(d)],key=lambda d:-legacy_score(c['query'],d))],
         'retrieval_only':[identity(d) for d in qdocs if kind=='author' or eligible(d)],
         'candidate_v1':[x[1] for x in rows],
         'without_cover':[x[1] for x in rank(c['query'],pool,qdocs,tdocs,kind,False)]}
  report={'id':cid,'query':c['query'],'split':c['split'],'candidateCount':len(pool),'metrics':{},'top':{}}
  for name,ids in paths.items():
   report['metrics'][name]=metrics(ids,label)
   report['top'][name]=[dict(id=k,title=(byid[k].get('title',byid[k].get('name'))) if k in byid else next((r['work']['title'] for r in live if identity(r['work'])==k),k)) for k in ids[:10]]
  report['explanations']=[dict(id=x[1],title=x[2].get('title',x[2].get('name')),score=round(x[0][1],4),parts=x[3]) for x in rows[:10]]
  if label.get('preferredStart'):
   report['preferredStartRank']=next((i+1 for i,x in enumerate(rows) if x[1]==label['preferredStart']),None)
  if label.get('editionISBN'):
   report['candidateEditionExact']=bool(rows and isbn13(label['editionISBN']) in {isbn13(i) for i in edition(rows[0][2]).get('isbn',[])})
  out.append(report)
 summary={}
 for group in ('all',*dict.fromkeys(c['split'] for c in cases)):
  rs=out if group=='all' else [r for r in out if r['split']==group]
  summary[group]={'queries':len(rs)}
  for method in out[0]['metrics']:
   summary[group][method]={m:round(sum(r['metrics'][method][m] for r in rs)/len(rs),4) for m in ('top1','hit10','rr10')}
 payload={'weightsVersion':WEIGHTS_VERSION,'rankerSHA256':hashlib.sha256((ROOT/'rank.py').read_bytes()).hexdigest(),'summary':summary,'queries':out}
 (ROOT/'results.json').write_text(json.dumps(payload,indent=2)+'\n')
 print(json.dumps(summary,indent=2))
 for r in out:print(r['id'],[(m,r['top'][m][0]['title'] if r['top'][m] else '(none)',r['metrics'][m]['targetRank']) for m in ('live','retrieval_only','candidate_v1')], 'starter',r.get('preferredStartRank'))
if __name__=='__main__':evaluate()
