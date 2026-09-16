#!/usr/bin/env python3
"""Choose candidate-only image publication; merges and tags never publish."""

import os
import re


def policy(event, requested, sha, run_id, attempt):
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("a full source commit is required")
    if not all(re.fullmatch(r"[1-9][0-9]*", value) for value in (run_id, attempt)):
        raise ValueError("positive run and attempt identifiers are required")
    if requested not in ("", "false", "true"):
        raise ValueError("publish_candidate must be a boolean")
    return {
        "publish": str(event == "workflow_dispatch" and requested == "true").lower(),
        "tag": f"candidate-{sha}-{run_id}-{attempt}",
    }


if __name__ == "__main__":
    result = policy(
        os.environ["GITHUB_EVENT_NAME"],
        os.environ.get("INPUT_PUBLISH_CANDIDATE", ""),
        os.environ["GITHUB_SHA"],
        os.environ["GITHUB_RUN_ID"],
        os.environ["GITHUB_RUN_ATTEMPT"],
    )
    with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
        for key, value in result.items():
            print(f"{key}={value}", file=output)
