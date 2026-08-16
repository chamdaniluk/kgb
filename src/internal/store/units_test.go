package store

import (
	"context"
	"testing"
)

func TestListUnitsHandlesRootAndChildParentNames(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	var korwilID int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('KORWIL-UNIT-TEST','Korwil Unit Test','korwil') RETURNING id`).Scan(&korwilID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO units (code,name,type,parent_id) VALUES ('SMP-UNIT-TEST','SMP Unit Test','smp',$1)`, korwilID); err != nil {
		t.Fatal(err)
	}
	units, err := ListUnits(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 {
		t.Fatalf("len(units) = %d, want 2", len(units))
	}
	for _, unit := range units {
		switch unit.Type {
		case "korwil":
			if unit.ParentName != "" {
				t.Errorf("root ParentName = %q, want empty", unit.ParentName)
			}
		case "smp":
			if unit.ParentName != "Korwil Unit Test" {
				t.Errorf("child ParentName = %q, want Korwil Unit Test", unit.ParentName)
			}
		}
	}
}

func TestGetUnitByIDHandlesRootParentName(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('KORWIL-UNIT-GET','Korwil Get Test','korwil') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	unit, err := GetUnitByID(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if unit.ParentName != "" {
		t.Fatalf("ParentName = %q, want empty", unit.ParentName)
	}
}

var _ = context.Background
