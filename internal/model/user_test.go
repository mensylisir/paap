package model

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestReplaceUserRolesHardDeletesExistingRowsBeforeInsert(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sql mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open gorm sql mock: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM "user_roles" WHERE user_id = $1`)).
		WithArgs(uint(7)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(`INSERT INTO "user_roles"`).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), uint(7), RolePlatformAdmin,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), uint(7), RoleAppAdmin,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	mock.ExpectCommit()

	roles, err := ReplaceUserRoles(db, 7, []string{RolePlatformAdmin, RoleAppAdmin})
	if err != nil {
		t.Fatalf("replace roles: %v", err)
	}
	if len(roles) != 2 || roles[0] != RolePlatformAdmin || roles[1] != RoleAppAdmin {
		t.Fatalf("roles = %#v", roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
