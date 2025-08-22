package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/surrealdb/surrealdb.go"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err.Error())
	}
}

func Main() error {
	var (
		addr     = flag.String("addr", "ws://localhost:8000/rpc", "SurrealDB address")
		user     = flag.String("user", "root", "Database user")
		pass     = flag.String("pass", "root", "Database password")
		ns       = flag.String("ns", "", "Namespace")
		db       = flag.String("db", "", "Database")
		varsFile = flag.String("vars", "", "Variables JSON file")
		exec     = flag.String("exec", "", "Query to execute")
	)
	flag.Parse()

	if *exec == "" {
		return errors.New("no query provided. Use -exec flag")
	}

	ctx := context.Background()

	conn, err := initConn(ctx, *addr, *user, *pass, *ns, *db)
	if err != nil {
		return fmt.Errorf("failed to initialize connection: %w", err)
	}

	vars, err := loadVars(*varsFile)
	if err != nil {
		return fmt.Errorf("failed to load variables: %w", err)
	}

	resp, err := surrealdb.Query[any](ctx, conn, *exec, vars)
	if err != nil {
		return fmt.Errorf("query failed: %v", err)
	}

	for _, result := range *resp {
		if result.Status == "OK" {
			switch v := result.Result.(type) {
			case []interface{}:
				for _, item := range v {
					if err := printResult(item); err != nil {
						return err
					}
				}
			case map[string]interface{}:
				if err := printResult(v); err != nil {
					return err
				}
			default:
				if v != nil {
					fmt.Printf("%v\n", v)
				}
			}
		} else {
			return result.Error
		}
	}

	return nil
}

func initConn(ctx context.Context, addr, user, pass, ns, db string) (*surrealdb.DB, error) {
	conn, err := surrealdb.FromEndpointURLString(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close(ctx)

	authData := &surrealdb.Auth{
		Username: user,
		Password: pass,
	}
	if _, err := conn.SignIn(ctx, authData); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	if ns != "" && db != "" {
		if err := conn.Use(ctx, ns, db); err != nil {
			return nil, fmt.Errorf("failed to use namespace/database: %v", err)
		}
	}

	return conn, nil
}

func loadVars(varsFile string) (map[string]any, error) {
	var vars map[string]any
	if varsFile != "" {
		jsonData, err := os.ReadFile(varsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables file: %v", err)
		}

		if err := json.Unmarshal(jsonData, &vars); err != nil {
			return nil, fmt.Errorf("failed to parse variables JSON: %v", err)
		}
	}

	return vars, nil
}

func printResult(item interface{}) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %v", err)
	}
	fmt.Printf("%s\n", data)
	return nil
}
