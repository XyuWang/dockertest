package dockertest

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
)

func mysqlHook(c *Container) (err error) {
	if c.Fresh {
		return
	}
	dsn := getDSN(c)
	if err = cleanMysql(dsn); err != nil {
		return
	}
	return initMysql(dsn)
}

func getDSN(c *Container) (dsn string) {
	var (
		user     = "root"
		host     = "127.0.0.1"
		pw, port string
	)
	envs := make(map[string]string)
	for _, env := range c.Env {
		a := strings.Split(env, "=")
		if len(a) == 2 {
			envs[a[0]] = a[1]
		}
	}
	if envs["MYSQL_ROOT_PASSWORD"] != "" {
		pw = envs["MYSQL_ROOT_PASSWORD"]
	}
	if envs["MYSQL_USER"] != "" {
		user = envs["MYSQL_USER"]
	}
	if envs["MYSQL_PASSWORD"] != "" {
		pw = envs["MYSQL_PASSWORD"]
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/?multiStatements=true", user, pw, host, port)
}

func sqlPath() (res string) {
	if os.Getenv("MYSQL_INIT_PATH") != "" {
		return os.Getenv("MYSQL_INIT_PATH")
	}
	dir, _ := os.Getwd()
	for filepath.Dir(dir) != "/" {
		if _, err := os.Stat(filepath.Join(dir, "resource")); err == nil {
			return filepath.Join(dir, "resource")
		}
		if _, err := os.Stat(filepath.Join(dir, "test")); err == nil {
			return filepath.Join(dir, "test")
		}
		dir = filepath.Dir(dir)
	}
	return
}

func cleanMysql(dsn string) (err error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	c := context.Background()
	defer db.Close()
	rows, err := db.QueryContext(c, "show databases")
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	var dbs []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		dbs = append(dbs, name)
	}
	dbs = businessDbs(dbs)
	for _, name := range dbs {
		_, err = db.ExecContext(c, fmt.Sprintf("drop database %s", name))
		if err != nil {
			err = errors.WithStack(err)
			return
		}
	}
	return
}

func initMysql(dsn string) (err error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	c := context.Background()
	defer db.Close()
	pathDir := sqlPath()
	files, err := ioutil.ReadDir(pathDir)
	if err != nil {
		err = errors.Wrapf(err, "read path: %s", pathDir)
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		content, err := ioutil.ReadFile(filepath.Join(pathDir, f.Name()))
		if err != nil {
			err = errors.Wrapf(err, "read %s", f.Name())
			return err
		}
		_, err = db.ExecContext(c, string(content))
		if err != nil {
			err = errors.Wrapf(err, "exec %s", f.Name())
			return err
		}
	}
	return
}

func businessDbs(dbs []string) (res []string) {
	for _, db := range dbs {
		if db == "information_schema" || db == "mysql" || db == "performance_schema" {
			continue
		}
		res = append(res, db)
	}
	return
}
