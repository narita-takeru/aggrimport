package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := "./aggregate_import.db"
	os.Create(dbPath)
	defer os.Remove(dbPath)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		panic(err)
	}

	defer db.Close()

	if _, err := db.Exec("CREATE TABLE imports (import varchar(255) NOT NULL PRIMARY KEY, cnt integer)"); err != nil {
		panic(err)
	}

	dirwalk(db, os.Args[1])
	dump(db)
}

func dump(db *sql.DB) {
	rows, err := db.Query("select import,cnt from imports order by cnt desc")
	if err != nil {
		panic(err)
	}

	defer rows.Close()

	fmt.Printf("import\treference count\n")
	for rows.Next() {
		var impt string
		var cnt int
		if err := rows.Scan(&impt, &cnt); err != nil {
			panic(err)
		}

		fmt.Printf("%s\t%d\n", impt, cnt)
	}

	if err := rows.Err(); err != nil {
		panic(err)
	}
}

func aggregateImport(db *sql.DB, filepath string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath, nil, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s[%s]\n", filepath, err.Error())
		return
	}

	for _, decl := range file.Decls {
		switch td := decl.(type) {
		case *ast.GenDecl:
			switch td.Tok {
			case token.IMPORT:
				for _, sp := range td.Specs {
					s := sp.(*ast.ImportSpec)
					trimed := strings.Replace(s.Path.Value, `"`, "", -1)
					var result sql.Result
					if result, err = db.Exec(fmt.Sprintf("update imports set cnt = cnt + 1 where import = '%s'", trimed)); err != nil {
						panic(err)
					}

					hits, _ := result.RowsAffected()
					if hits <= 0 {
						if _, err := db.Exec(fmt.Sprintf("insert into imports(import, cnt) values('%s', 1)", trimed)); err != nil {
							panic(err)
						}
					}
				}
			}
		}
	}
}

func dirwalk(db *sql.DB, dir string) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		if file.IsDir() {
			dirwalk(db, filepath.Join(dir, file.Name()))
			continue
		}

		if strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		if strings.HasSuffix(file.Name(), ".go") {
			aggregateImport(db, filepath.Join(dir, file.Name()))
		}
	}
}
