// pattern: Imperative Shell
package provider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

func WriteProductSurfaceDiagnosticAuthorization(path string, authorization ProductSurfaceDiagnosticAuthorization) error {
	if err := ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(authorization, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal product diagnostic authorization: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create one-use product diagnostic authorization: %w", err)
	}
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func ReadProductSurfaceDiagnosticAuthorization(path string) (ProductSurfaceDiagnosticAuthorization, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ProductSurfaceDiagnosticAuthorization{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var authorization ProductSurfaceDiagnosticAuthorization
	if err = decoder.Decode(&authorization); err != nil {
		return ProductSurfaceDiagnosticAuthorization{}, fmt.Errorf("decode product diagnostic authorization: %w", err)
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ProductSurfaceDiagnosticAuthorization{}, errors.New("product diagnostic authorization has trailing data")
	}
	if err = ValidateProductSurfaceDiagnosticAuthorization(authorization, authorization); err != nil {
		return ProductSurfaceDiagnosticAuthorization{}, err
	}
	return authorization, nil
}

func ReserveProductSurfaceDiagnosticOnce(authorizationPath, reservationPath string, expected ProductSurfaceDiagnosticAuthorization, reservedAt time.Time) (ProductSurfaceDiagnosticReservation, error) {
	if err := ValidateProductSurfaceDiagnosticAuthorization(expected, expected); err != nil {
		return ProductSurfaceDiagnosticReservation{}, err
	}
	bound, err := ReadProductSurfaceDiagnosticAuthorization(authorizationPath)
	if err != nil {
		return ProductSurfaceDiagnosticReservation{}, err
	}
	if err = ValidateProductSurfaceDiagnosticAuthorization(bound, expected); err != nil {
		return ProductSurfaceDiagnosticReservation{}, err
	}
	raw, err := json.Marshal(bound)
	if err != nil {
		return ProductSurfaceDiagnosticReservation{}, err
	}
	digest := sha256.Sum256(raw)
	reservation := ProductSurfaceDiagnosticReservation{
		SchemaVersion:   ProductSurfaceDiagnosticReservationSchema,
		AuthorizationID: bound.AuthorizationID, AuthorizationDigest: hex.EncodeToString(digest[:]),
		ReservationNumber: 1, ProviderAttemptLimit: 1, RetryLimit: 0, ReservedAt: reservedAt.UTC(),
	}
	reservationRaw, err := json.MarshalIndent(reservation, "", "  ")
	if err != nil {
		return ProductSurfaceDiagnosticReservation{}, err
	}
	file, err := os.OpenFile(reservationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ProductSurfaceDiagnosticReservation{}, errors.New("product surface diagnostic reservation already consumed; no retry")
		}
		return ProductSurfaceDiagnosticReservation{}, fmt.Errorf("claim product surface diagnostic reservation: %w", err)
	}
	if _, err = file.Write(reservationRaw); err != nil {
		_ = file.Close()
		return ProductSurfaceDiagnosticReservation{}, err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return ProductSurfaceDiagnosticReservation{}, err
	}
	if err = file.Close(); err != nil {
		return ProductSurfaceDiagnosticReservation{}, err
	}
	return reservation, nil
}
