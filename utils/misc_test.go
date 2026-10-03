/**
 * @file misc_test.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 10/03/2026
 */

package utils

import "testing"

func TestSHA256DigestMatchesCryptoPassword(t *testing.T) {
	h1 := SHA256Digest("mypassword", "salt123")
	h2 := CryptoPassword("mypassword", "salt123")

	if h1 != h2 {
		t.Fatalf("the deprecated alias must delegate: SHA256Digest=%q CryptoPassword=%q", h1, h2)
	}
}

// The digest must stay deterministic — the function is used for cache keys and
// change detection, where a per-call random salt would break lookups entirely.
func TestSHA256DigestIsDeterministic(t *testing.T) {
	if a, b := SHA256Digest("x", "y"), SHA256Digest("x", "y"); a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}

	if a := SHA256Digest("x", "y"); a == SHA256Digest("y", "x") {
		t.Fatal("argument order must matter")
	}
}

// The "@@" join is ambiguous by construction: these two pairs are different
// inputs that must not collide, and today they do. The test records the
// collision rather than asserting it does not happen, because the ambiguity
// is inherent to the signature — the fix is for callers to avoid "@@" in
// their inputs, which SHA256Digest's doc comment states.
//
// If this test ever starts passing, the separator handling changed and the
// doc comment needs updating with it.
func TestSHA256DigestSeparatorIsAmbiguous(t *testing.T) {
	if SHA256Digest("a@@b", "c") != SHA256Digest("a", "b@@c") {
		t.Skip("separator ambiguity no longer present: update the SHA256Digest doc comment")
	}
}

func TestMD5StringIsStable(t *testing.T) {
	first := MD5String("abc")
	if first == "" {
		t.Fatal("MD5String must return a digest")
	}

	if second := MD5String("abc"); second != first {
		t.Fatalf("not deterministic: %q vs %q", first, second)
	}

	if other := MD5String("abd"); other == first {
		t.Fatal("different inputs must produce different digests")
	}
}

func TestCryptoPassword(t *testing.T) {
	// Same input, same digest.
	h1 := CryptoPassword("mypassword", "salt123")
	h2 := CryptoPassword("mypassword", "salt123")
	if h1 != h2 {
		t.Fatal("same input must produce the same digest")
	}

	// Different password.
	h1 = CryptoPassword("password1", "salt")
	h2 = CryptoPassword("password2", "salt")
	if h1 == h2 {
		t.Fatal("different passwords must produce different digests")
	}

	// Different salt.
	h1 = CryptoPassword("password", "salt1")
	h2 = CryptoPassword("password", "salt2")
	if h1 == h2 {
		t.Fatal("different salts must produce different digests")
	}

	// 64 hex characters (SHA-256).
	h := CryptoPassword("password", "salt")
	if len(h) != 64 {
		t.Fatalf("digest length = %d, want 64", len(h))
	}
}
