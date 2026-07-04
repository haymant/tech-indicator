package repository

import (
	"strings"
	"testing"
)

func TestPostgresDialect_CreateTable(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.CreateTable()

	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS snapshots") {
		t.Errorf("CreateTable should contain CREATE TABLE IF NOT EXISTS snapshots, got: %s", sql)
	}
	if !strings.Contains(sql, "name   TEXT") {
		t.Errorf("CreateTable should contain name TEXT, got: %s", sql)
	}
	if !strings.Contains(sql, "date   DATE") {
		t.Errorf("CreateTable should contain date DATE, got: %s", sql)
	}
	if !strings.Contains(sql, "open   DOUBLE PRECISION") {
		t.Errorf("CreateTable should contain open DOUBLE PRECISION, got: %s", sql)
	}
	if !strings.Contains(sql, "high   DOUBLE PRECISION") {
		t.Errorf("CreateTable should contain high DOUBLE PRECISION, got: %s", sql)
	}
	if !strings.Contains(sql, "low    DOUBLE PRECISION") {
		t.Errorf("CreateTable should contain low DOUBLE PRECISION, got: %s", sql)
	}
	if !strings.Contains(sql, "close  DOUBLE PRECISION") {
		t.Errorf("CreateTable should contain close DOUBLE PRECISION, got: %s", sql)
	}
	if !strings.Contains(sql, "volume DOUBLE PRECISION") {
		t.Errorf("CreateTable should contain volume DOUBLE PRECISION, got: %s", sql)
	}
}

func TestPostgresDialect_DropTable(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.DropTable()

	if sql != "DROP TABLE IF EXISTS snapshots" {
		t.Errorf("DropTable expected 'DROP TABLE IF EXISTS snapshots', got: %s", sql)
	}
}

func TestPostgresDialect_Assets(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.Assets()

	if sql != "SELECT DISTINCT name FROM snapshots ORDER BY name" {
		t.Errorf("Assets expected 'SELECT DISTINCT name FROM snapshots ORDER BY name', got: %s", sql)
	}
}

func TestPostgresDialect_GetSince(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.GetSince()

	if !strings.Contains(sql, "SELECT") {
		t.Errorf("GetSince should contain SELECT, got: %s", sql)
	}
	if !strings.Contains(sql, "date, open, high, low, close, volume") {
		t.Errorf("GetSince should contain the column list, got: %s", sql)
	}
	if !strings.Contains(sql, "FROM snapshots") {
		t.Errorf("GetSince should contain FROM snapshots, got: %s", sql)
	}
	if !strings.Contains(sql, "WHERE name = $1") {
		t.Errorf("GetSince should use $1 placeholder for name, got: %s", sql)
	}
	if !strings.Contains(sql, "date >= $2") {
		t.Errorf("GetSince should use $2 placeholder for date, got: %s", sql)
	}
	if !strings.Contains(sql, "ORDER BY date") {
		t.Errorf("GetSince should contain ORDER BY date, got: %s", sql)
	}
}

func TestPostgresDialect_LastDate(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.LastDate()

	if sql != "SELECT MAX(date) FROM snapshots WHERE name = $1" {
		t.Errorf("LastDate expected 'SELECT MAX(date) FROM snapshots WHERE name = $1', got: %s", sql)
	}
}

func TestPostgresDialect_CreateIndex(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.CreateIndex()

	if !strings.Contains(sql, "CREATE INDEX IF NOT EXISTS idx_snapshots_name_date") {
		t.Errorf("CreateIndex should contain the index definition, got: %s", sql)
	}
	if !strings.Contains(sql, "ON snapshots (name, date)") {
		t.Errorf("CreateIndex should be on (name, date), got: %s", sql)
	}
}

func TestPostgresDialect_Append(t *testing.T) {
	d := NewPostgresDialect()
	sql := d.Append()

	if !strings.Contains(sql, "INSERT INTO snapshots") {
		t.Errorf("Append should contain INSERT INTO snapshots, got: %s", sql)
	}
	if !strings.Contains(sql, "$1, $2, $3, $4, $5, $6, $7") {
		t.Errorf("Append should have 7 $N placeholders, got: %s", sql)
	}
	if !strings.Contains(sql, "name, date, open, high, low, close, volume") {
		t.Errorf("Append should contain all 7 column names, got: %s", sql)
	}
}
