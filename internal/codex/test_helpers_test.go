// pattern: Functional Core
package codex

import "strings"

func validTransportPolicyAuth() AuthFingerprintManifest {
	return AuthFingerprintManifest{
		AuthSourceClass:                                "mounted_codex_auth_file",
		AuthIdentityFingerprintSchemaVersion:           AuthIdentityFingerprintSchemaVersion,
		AuthIdentityFingerprint:                        strings.Repeat("a", 64),
		AuthIdentityFingerprintStatus:                  "available",
		AuthCredentialRevisionFingerprintSchemaVersion: AuthCredentialRevisionFingerprintSchemaVersion,
		AuthCredentialRevisionFingerprint:              strings.Repeat("b", 64),
		AuthCredentialRevisionFingerprintStatus:        "available",
	}
}
