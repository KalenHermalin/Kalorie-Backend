"""Port of internal/utils/rand_string_test.go"""

import base64

from app.utils import generate_secure_string


def test_generate_secure_string():
    # Test that it generates the correct length
    length = 32
    s = generate_secure_string(length)
    decoded = base64.urlsafe_b64decode(s)
    assert len(decoded) == length

    # Test for uniqueness (call it twice, they shouldn't match)
    s2 = generate_secure_string(length)
    assert s != s2
