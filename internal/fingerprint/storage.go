package fingerprint

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

const fingerprintSchemaSQL = `
CREATE TABLE IF NOT EXISTS incident_fingerprints (
    incident_id  TEXT PRIMARY KEY,
    vector       BLOB NOT NULL,
    features     TEXT NOT NULL,
    generated_at DATETIME NOT NULL,
    version      INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_fingerprints_generated ON incident_fingerprints(generated_at);
`

func initDB(db *sql.DB) error {
	_, err := db.Exec(fingerprintSchemaSQL)
	if err != nil {
		return fmt.Errorf("fingerprint: init schema: %w", err)
	}
	return nil
}

func saveFingerprint(db *sql.DB, fp *IncidentFingerprint) error {
	vectorBytes := vectorToBytes(fp.Vector)
	featuresJSON, err := json.Marshal(fp.Features)
	if err != nil {
		return fmt.Errorf("fingerprint: marshal features: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO incident_fingerprints (incident_id, vector, features, generated_at, version)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(incident_id) DO UPDATE SET
		     vector = excluded.vector,
		     features = excluded.features,
		     generated_at = excluded.generated_at,
		     version = excluded.version`,
		fp.IncidentID, vectorBytes, string(featuresJSON), fp.GeneratedAt, fp.Version,
	)
	if err != nil {
		return fmt.Errorf("fingerprint: save: %w", err)
	}
	return nil
}

func loadFingerprint(db *sql.DB, incidentID string) (*IncidentFingerprint, error) {
	row := db.QueryRow(
		`SELECT incident_id, vector, features, generated_at, version
		 FROM incident_fingerprints WHERE incident_id = ?`, incidentID,
	)

	var fp IncidentFingerprint
	var vectorBlob []byte
	var featuresJSON string

	if err := row.Scan(&fp.IncidentID, &vectorBlob, &featuresJSON, &fp.GeneratedAt, &fp.Version); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("fingerprint: load: %w", err)
	}

	vec, err := bytesToVector(vectorBlob)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: decode vector: %w", err)
	}
	fp.Vector = vec

	if err := json.Unmarshal([]byte(featuresJSON), &fp.Features); err != nil {
		return nil, fmt.Errorf("fingerprint: unmarshal features: %w", err)
	}

	return &fp, nil
}

func loadAllFingerprints(db *sql.DB) (map[string]*IncidentFingerprint, error) {
	rows, err := db.Query(
		`SELECT incident_id, vector, features, generated_at, version
		 FROM incident_fingerprints ORDER BY generated_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("fingerprint: load all: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*IncidentFingerprint)
	for rows.Next() {
		var fp IncidentFingerprint
		var vectorBlob []byte
		var featuresJSON string

		if err := rows.Scan(&fp.IncidentID, &vectorBlob, &featuresJSON, &fp.GeneratedAt, &fp.Version); err != nil {
			continue
		}

		vec, err := bytesToVector(vectorBlob)
		if err != nil {
			continue
		}
		fp.Vector = vec

		if err := json.Unmarshal([]byte(featuresJSON), &fp.Features); err != nil {
			continue
		}

		result[fp.IncidentID] = &fp
	}

	return result, nil
}

func vectorToBytes(v [VectorSize]float64) []byte {
	buf := make([]byte, VectorSize*8)
	for i, f := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}
	return buf
}

func bytesToVector(b []byte) ([VectorSize]float64, error) {
	var v [VectorSize]float64
	if len(b) != VectorSize*8 {
		return v, fmt.Errorf("invalid vector blob size: %d, expected %d", len(b), VectorSize*8)
	}
	for i := 0; i < VectorSize; i++ {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return v, nil
}
