#!/usr/bin/env python3
"""Exercise the publication policy used directly by GitHub Actions."""

import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("publication", Path(__file__).with_name("image-publication.py"))
publication = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publication)


class PublicationPolicyTest(unittest.TestCase):
    def result(self, event="workflow_dispatch", requested="false", **identity):
        return publication.policy(event, requested, identity.get("sha", "a" * 40),
                                  identity.get("run_id", "123"), identity.get("attempt", "1"))

    def test_only_explicit_dispatch_publishes(self):
        for event in ("push", "pull_request", "pull_request_target", "release", "schedule", "workflow_dispatch"):
            for requested in ("", "false", "true"):
                with self.subTest(event=event, requested=requested):
                    self.assertEqual(self.result(event, requested)["publish"],
                                     "true" if (event, requested) == ("workflow_dispatch", "true") else "false")

    def test_tags_are_candidates_with_full_source_identity(self):
        self.assertEqual(self.result(requested="true")["tag"], f"candidate-{'a' * 40}-123-1")

    def test_rebuilds_do_not_reuse_tags(self):
        tags = {self.result(**values)["tag"] for values in (
            {}, {"attempt": "2"}, {"run_id": "124"}, {"sha": "b" * 40})}
        self.assertEqual(len(tags), 4)

    def test_malformed_identity_or_request_fails_closed(self):
        for values in ({"sha": "main"}, {"sha": "latest\npublish=true"},
                       {"run_id": "0"}, {"run_id": "1\npublish=true"},
                       {"attempt": ""}, {"requested": "yes"}):
            with self.subTest(values=values), self.assertRaises(ValueError):
                self.result(**values)


if __name__ == "__main__":
    unittest.main()
