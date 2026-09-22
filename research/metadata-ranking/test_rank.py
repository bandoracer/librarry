import unittest
from rank import isbn13,rank,score,eligible

def doc(key='OL1W',cover=0):
 return {'key':key,'title':'A Quiet Book','author_name':['Author'], 'isbn':['9781423103349'], 'edition_count':1, 'cover_i':cover, 'editions':{'docs':[{'language':['eng']}]}}

class RankContracts(unittest.TestCase):
 def test_isbn_checksum_and_equivalence(self):
  self.assertEqual(isbn13('1423103343'),isbn13('978-1-4231-0334-9'))
  self.assertIsNone(isbn13('9781423103348'))
  self.assertIsNone(isbn13('9781423103349 extra'))
 def test_missing_cover_does_not_beat_identifier(self):
  exact=doc();wrong=doc('OL2W',900);wrong['isbn']=['9780593135204']
  rows=rank('9781423103349',[wrong,exact],[wrong,exact])
  self.assertEqual(rows[0][1],'OL1W')
 def test_known_english_edition_not_rejected_by_work_language(self):
  d=doc();d['language']=['fin','eng'];self.assertTrue(eligible(d))
  d['editions']['docs'][0]['language']=['fin'];self.assertFalse(eligible(d))
  d['editions']['docs'][0].pop('language');self.assertTrue(eligible(d))
 def test_explicit_summary_request_not_penalized(self):
  d=doc();d['title']='Summary of A Quiet Book'
  self.assertIn('unrequested_derivative',score('a quiet book',d,1)[1])
  self.assertNotIn('unrequested_derivative',score('summary of a quiet book',d,1)[1])
 def test_cover_weight_bounded(self):
  a=score('a quiet book',doc(),1)[0][1]
  b=score('a quiet book',doc(cover=900),1)[0][1]
  self.assertEqual(b-a,1)
 def test_deterministic_ties(self):
  a,b=doc('OL1W'),doc('OL2W')
  self.assertEqual([x[1] for x in rank('book',[b,a],[])],['OL1W','OL2W'])

if __name__=='__main__':unittest.main()
