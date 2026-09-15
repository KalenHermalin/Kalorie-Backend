"""Port of internal/env/env.go.

Note: just like the Go version, main.py does NOT use this helper - it reads
os.environ directly with its own error handling. Kept as-is (unused) for
fidelity to the original.
"""

import os


def get_string(key: str) -> str:
    val = os.environ.get(key)
    if val is None:
        print(f"Could not find key: {key}")
        raise KeyError(f"Could not find key: {key}")
    return val
