package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time" // Import time package for potential delays

	"github.com/jackc/pgx/v5"
	//"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
)

// Config struct to hold database configuration
type Config struct {
	Host        string
	Port        uint64
	User        string
	Password    string
	DBName      string // Database to connect initially (e.g., "postgres")
	TargetDB    string // Database to be created/managed
	SQLFile     string
	SSLMode     string
	SSLCert     string
	SSLKey      string
	SSLRootCert string
}

// LoadConfig reads configuration from .env file and environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Println("Cảnh báo: Không thể tải file .env:", err) // Warning in Vietnamese
		}
	}

	// Read environment variables
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")

	// Check required variables
	if host == "" || user == "" || password == "" {
		return nil, fmt.Errorf("lỗi: Thiếu thông tin kết nối trong file .env hoặc biến môi trường! Cần có: DB_HOST, DB_USER, DB_PASSWORD") // Error in Vietnamese
	}

	// Read optional variables with defaults
	portStr := os.Getenv("DB_PORT")
	if portStr == "" {
		portStr = "5432"
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("lỗi: DB_PORT không hợp lệ: %w", err) // Error in Vietnamese
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres" // Default database to connect to for admin tasks
	}

	targetDB := os.Getenv("TARGET_DB")
	if targetDB == "" {
		targetDB = "ticket_selling_app" // Default target database name
	}

	sqlFile := os.Getenv("SQL_FILE")
	if sqlFile == "" {
		sqlFile = "sql.sql"
	}

	sslMode := os.Getenv("SSL_MODE")
	if sslMode == "" {
		sslMode = "require" // Default SSL mode
	}

	cfg := &Config{
		Host:        host,
		Port:        port,
		User:        user,
		Password:    password,
		DBName:      dbName,
		TargetDB:    targetDB,
		SQLFile:     sqlFile,
		SSLMode:     sslMode,
		SSLCert:     os.Getenv("SSL_CERT"),
		SSLKey:      os.Getenv("SSL_KEY"),
		SSLRootCert: os.Getenv("SSL_ROOT_CERT"),
	}

	return cfg, nil
}

// buildDSN constructs the Data Source Name string for pgx
func buildDSN(cfg *Config, dbName string) string {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, dbName, cfg.SSLMode)

	// Append SSL parameters only if SSL is not disabled
	if cfg.SSLMode != "disable" {
		if cfg.SSLCert != "" {
			dsn += fmt.Sprintf(" sslcert=%s", cfg.SSLCert)
		}
		if cfg.SSLKey != "" {
			dsn += fmt.Sprintf(" sslkey=%s", cfg.SSLKey)
		}
		if cfg.SSLRootCert != "" {
			dsn += fmt.Sprintf(" sslrootcert=%s", cfg.SSLRootCert)
		}
	}
	return dsn
}

func main() {
	fmt.Println("=== Triển khai database Ticket Selling App lên Google Cloud SQL (Go Version) ===")

	// 1. Load Configuration
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Lỗi cấu hình: %v", err) // Error in Vietnamese
	}
	fmt.Println("Sử dụng thông số kết nối:")
	fmt.Printf("Host: %s, Port: %d, User: %s, Database ban đầu: %s, Database đích: %s\n",
		cfg.Host, cfg.Port, cfg.User, cfg.DBName, cfg.TargetDB)

	// 2. Check SQL file existence
	if _, err := os.Stat(cfg.SQLFile); os.IsNotExist(err) {
		log.Fatalf("Lỗi: File SQL '%s' không tồn tại. File này nên chứa các lệnh CREATE TABLE, INSERT, etc. cho database '%s', KHÔNG chứa CREATE DATABASE hoặc lệnh \\c.", cfg.SQLFile, cfg.TargetDB) // Error in Vietnamese
	}
	fmt.Printf("Sử dụng file SQL: %s (dự kiến chứa schema/data cho %s)\n", cfg.SQLFile, cfg.TargetDB)

	// 3. Establish initial connection (to default DB like 'postgres')
	ctx := context.Background()
	adminDSN := buildDSN(cfg, cfg.DBName) // Connect to admin DB (e.g., postgres)
	adminConn, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		log.Fatalf("Lỗi: Không thể kết nối đến database quản trị '%s'. Kiểm tra cấu hình kết nối.\n%v", cfg.DBName, err) // Error in Vietnamese
	}
	// Use a separate context for closing connection to avoid issues if main ctx is cancelled
	defer adminConn.Close(context.Background())

	fmt.Println("Kiểm tra kết nối đến PostgreSQL (database quản trị)...")
	var test int
	err = adminConn.QueryRow(ctx, "SELECT 1").Scan(&test)
	if err != nil {
		log.Fatalf("Lỗi: Kiểm tra kết nối thất bại.\n%v", err) // Error in Vietnamese
	}
	fmt.Println("Kết nối đến database quản trị thành công.")

	// 4. Check if target database exists
	fmt.Printf("Kiểm tra database %s...\n", cfg.TargetDB)
	var exists bool
	checkDbQuery := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err = adminConn.QueryRow(ctx, checkDbQuery, cfg.TargetDB).Scan(&exists)
	if err != nil {
		log.Fatalf("Lỗi khi kiểm tra sự tồn tại của database '%s': %v", cfg.TargetDB, err) // Error in Vietnamese
	}

	databaseExistedInitially := exists // Keep track if the DB existed before any action

	// 5. Handle existing database (Drop and Recreate confirmation)
	if exists {
		fmt.Printf("Database %s đã tồn tại.\n", cfg.TargetDB)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Bạn có muốn xóa và tạo lại database? (y/n): ")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(strings.ToLower(confirm))

		if confirm == "y" {
			fmt.Println("Đang xóa database cũ...")
			// IMPORTANT: Terminate connections before dropping
			// Use pgx.Identifier for safety with the database name in the query string
			terminateQuery := fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = %s;", pgx.Identifier{cfg.TargetDB}.Sanitize())
			_, terminateErr := adminConn.Exec(ctx, terminateQuery)
			if terminateErr != nil {
				// Log warning but proceed, dropping might still work or fail with a clearer message
				log.Printf("Cảnh báo: Không thể ngắt kết nối đến database '%s', việc xóa có thể thất bại: %v", cfg.TargetDB, terminateErr) // Warning in Vietnamese
			} else {
				fmt.Println("Đã gửi yêu cầu ngắt kết nối đến database cũ.")
				// Optional: Add a small delay to allow connections to terminate
				time.Sleep(2 * time.Second)
			}

			// Use Identifier for safety when dropping
			dropQuery := fmt.Sprintf("DROP DATABASE %s;", pgx.Identifier{cfg.TargetDB}.Sanitize())
			_, err = adminConn.Exec(ctx, dropQuery)
			if err != nil {
				log.Fatalf("Lỗi khi xóa database '%s': %v", cfg.TargetDB, err) // Error in Vietnamese
			}
			fmt.Println("Đã xóa database cũ.")
			exists = false // Mark as not existing anymore for the creation step
		} else {
			fmt.Println("Giữ lại database hiện có. Sẽ thực thi file SQL trên database này.")
			// Keep 'exists = true'
		}
	}

	// 6. Create target database if it doesn't exist (or was just dropped)
	if !exists {
		fmt.Printf("Đang tạo database %s...\n", cfg.TargetDB)
		// Use Identifier for safety when creating
		createQuery := fmt.Sprintf("CREATE DATABASE %s;", pgx.Identifier{cfg.TargetDB}.Sanitize())
		_, err = adminConn.Exec(ctx, createQuery)
		if err != nil {
			log.Fatalf("Lỗi khi tạo database '%s': %v", cfg.TargetDB, err) // Error in Vietnamese
		}
		fmt.Printf("Đã tạo database %s.\n", cfg.TargetDB)
		// Add a small delay after creating the database, sometimes needed for it to become fully available
		fmt.Println("Đợi một chút để database sẵn sàng...")
		time.Sleep(3 * time.Second)
	} else if databaseExistedInitially {
		// If the database existed and user chose not to drop, still proceed to run SQL script
		fmt.Printf("Cảnh báo: Database '%s' đã tồn tại và không bị xóa. Thực thi file SQL '%s' trên database hiện có.\n", cfg.TargetDB, cfg.SQLFile) // Warning in Vietnamese
	}

	// Close the admin connection NOW before connecting to the target DB
	fmt.Println("Đóng kết nối đến database quản trị.")
	adminConn.Close(context.Background()) // Use background context for closing

	// 7. Connect to the *target* database
	fmt.Printf("Đang kết nối đến database đích '%s'...\n", cfg.TargetDB)
	targetDSN := buildDSN(cfg, cfg.TargetDB)
	var targetConn *pgx.Conn
	var connectErr error
	// Retry connecting to the target database a few times as it might take a moment to initialize
	for i := 0; i < 5; i++ {
		targetConn, connectErr = pgx.Connect(ctx, targetDSN)
		if connectErr == nil {
			break // Success
		}
		log.Printf("Kết nối đến '%s' thất bại (lần %d), đang thử lại... Lỗi: %v", cfg.TargetDB, i+1, connectErr)
		time.Sleep(time.Duration(i+1) * 2 * time.Second) // Exponential backoff
	}
	if connectErr != nil {
		log.Fatalf("Lỗi: Không thể kết nối đến database mới '%s' sau khi tạo/kiểm tra (đã thử nhiều lần). Lỗi cuối cùng: %v", cfg.TargetDB, connectErr) // Error in Vietnamese
	}
	defer targetConn.Close(context.Background()) // Use background context for closing
	fmt.Printf("Kết nối đến database đích '%s' thành công.\n", cfg.TargetDB)

	// 8. Execute SQL script content against the target database
	fmt.Printf("Đang thực thi nội dung file SQL (%s) trên database '%s'...\n", cfg.SQLFile, cfg.TargetDB)
	sqlBytes, err := ioutil.ReadFile(cfg.SQLFile)
	if err != nil {
		log.Fatalf("Lỗi khi đọc file SQL '%s': %v", cfg.SQLFile, err) // Error in Vietnamese
	}
	sqlScript := string(sqlBytes)

	// IMPORTANT: Check if the script is empty or contains only whitespace
	if strings.TrimSpace(sqlScript) == "" {
		fmt.Printf("Cảnh báo: File SQL '%s' trống hoặc chỉ chứa khoảng trắng. Bỏ qua thực thi script.\n", cfg.SQLFile) // Warning in Vietnamese
	} else {
		// Execute the script. pgx doesn't support multiple statements in a single Exec by default
		// unless the driver specifically handles it or they are separated by semicolons
		// in a way the server understands. For complex scripts, consider a migration tool
		// or splitting the script. However, Exec often handles simple semicolon-separated statements.
		_, err = targetConn.Exec(ctx, sqlScript)
		if err != nil {
			// Provide more context on the error
			log.Fatalf("Lỗi khi thực thi script SQL từ file '%s' trên database '%s'. Kiểm tra cú pháp SQL trong file (KHÔNG dùng lệnh \\ psql). Lỗi: %v", cfg.SQLFile, cfg.TargetDB, err) // Error in Vietnamese
		}
		fmt.Println("Thực thi script SQL thành công!")
	}

	// 9. Verify table creation (using targetConn)
	fmt.Println("Kiểm tra các bảng đã được tạo...")
	var tableCount int
	countQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'"
	err = targetConn.QueryRow(ctx, countQuery).Scan(&tableCount)
	if err != nil {
		log.Fatalf("Lỗi khi đếm bảng trong database '%s': %v", cfg.TargetDB, err) // Error in Vietnamese
	}
	fmt.Printf("Tìm thấy %d bảng trong schema 'public' của database '%s'.\n", tableCount, cfg.TargetDB)

	// List tables
	if tableCount > 0 {
		fmt.Println("Danh sách các bảng:")
		listQuery := "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name"
		rows, err := targetConn.Query(ctx, listQuery)
		if err != nil {
			log.Fatalf("Lỗi khi liệt kê bảng trong database '%s': %v", cfg.TargetDB, err) // Error in Vietnamese
		}
		// Ensure rows are closed. Use a separate context for safety.
		defer rows.Close()

		tableFound := false
		for rows.Next() {
			tableFound = true
			var tableName string
			if err := rows.Scan(&tableName); err != nil {
				// Log error but continue trying to list others
				log.Printf("Lỗi khi đọc tên bảng: %v", err) // Error in Vietnamese
				continue
			}
			fmt.Printf("- %s\n", tableName)
		}

		if err := rows.Err(); err != nil { // Check for errors during iteration
			log.Fatalf("Lỗi sau khi lặp qua các hàng bảng: %v", err) // Error in Vietnamese
		}
		if !tableFound && tableCount > 0 {
			// This case should ideally not happen if count > 0, but good to check
			fmt.Println("Đã đếm được bảng nhưng không thể liệt kê tên.")
		}

	} else {
		fmt.Println("Không tìm thấy bảng nào trong schema 'public'.")
	}

	fmt.Println("Hoàn tất triển khai!")
}
