package store

// ImportedStaffUser adalah akun petugas dari daftar resmi Dinas.
type ImportedStaffUser struct {
	Username             string
	Password             string
	Role                 string
	Name                 string
	UnitCode             string
	UnitName             string
	UnitType             string
	UnitDistrict         string
	NIK                  string
	SignatureImageBase64 string
	EmployeeNumber       string
	JobTitle             string
	NeedsSignerProfile   bool
	ParentUnitCode       string
	ParentUnitName       string
}
