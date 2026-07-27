"""Send a prompt to a local Ollama server and validate the response with Pydantic."""

from __future__ import annotations

import json
import os
import re
import sys
import urllib.error
import urllib.request

from pydantic import ValidationError

from .schema import TaskList

OLLAMA_HOST = os.environ.get("OLLAMA_HOST", "http://localhost:11434")
OLLAMA_MODEL = os.environ.get("OLLAMA_MODEL", "gemma4:latest")
DEFAULT_PROMPT_FILE = (
    "/home/michel/projects/repos/workspaces/jets_ws/data/"
    "patient_clinical_summary/initial_test_prompt.md"
)


def parse_prompt(text: str) -> tuple[str, str]:
    """Split a prompt file into its ``System:`` and ``User:`` sections."""
    match = re.match(r"\s*System:\s*(.*?)\n\s*User:\s*(.*)", text, re.DOTALL)
    if not match:
        raise ValueError("Prompt file must contain a 'System:' and a 'User:' section.")
    return match.group(1).strip(), match.group(2).strip()


def call_ollama(system: str, user: str) -> str:
    """Call the Ollama /api/chat endpoint and return the message content."""
    payload = {
        "model": OLLAMA_MODEL,
        "stream": False,
        # Ask Ollama to constrain generation to our Pydantic-derived JSON Schema.
        "format": TaskList.json_schema(),
        "options": {"temperature": 0},
        "messages": [
            {"role": "system", "content": system},
            {"role": "user", "content": user},
        ],
    }
    # Print the payload for debugging purposes
    print("Payload sent to Ollama:\n", json.dumps(payload, indent=2), file=sys.stderr)

    request = urllib.request.Request(
        f"{OLLAMA_HOST}/api/chat",
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request) as response:
        data = json.loads(response.read().decode("utf-8"))

    if "error" in data:
        raise RuntimeError(f"Ollama error: {data['error']}")
    return data.get("message", {}).get("content", "")


def main() -> int:
    prompt_file = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_PROMPT_FILE
    with open(prompt_file, "r", encoding="utf-8") as handle:
        system, user = parse_prompt(handle.read())

    print(f"Model      : {OLLAMA_MODEL}")
    print(f"Ollama host: {OLLAMA_HOST}")
    print(f"Prompt file: {prompt_file}\n")

    try:
        content = call_ollama(system, user)
    except urllib.error.URLError as err:
        print(f"Failed to reach Ollama at {OLLAMA_HOST}: {err}", file=sys.stderr)
        return 1

    try:
        tasks = TaskList.validate_json(content)
    except ValidationError as err:
        print("Model output failed schema validation:\n", file=sys.stderr)
        print(err, file=sys.stderr)
        print("\nRaw output:\n" + content, file=sys.stderr)
        return 1

    dumped = TaskList.dump_python(tasks, mode="json")
    print(f"Validated {len(tasks)} task(s):\n")
    print(json.dumps(dumped, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
