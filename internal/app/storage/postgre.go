package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/models"
)

var (
	ErrUsernameNotUnique                       = errors.New("username is already in use")
	ErrUserNotFound                            = errors.New("user not found")
	ErrOrderNumberWasAlreadyAddedByThisUser    = errors.New("order number has already been added by this user")
	ErrOrderNumberWasAlreadyAddedByAnotherUser = errors.New("order number has already been added by another user")
)

type Storage struct {
	DB *sql.DB
}

func Initialize(uri string) (*Storage, error) {
	db, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, fmt.Errorf("initialize: error opening database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("initialize: error verifing database connection: %w", err)
	}

	err = createIfNotExists(db)
	if err != nil {
		return nil, fmt.Errorf("initialize: error creating database structure: %w", err)
	}
	return &Storage{DB: db}, nil
}

func createIfNotExists(db *sql.DB) error {
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS users (
-- 			id SERIAL PRIMARY KEY,
			user_id VARCHAR PRIMARY KEY NOT NULL,
			login VARCHAR NOT NULL,
			password VARCHAR NOT NULL,
			UNIQUE (user_id)
		);
		CREATE TABLE IF NOT EXISTS orders (
		    order_id VARCHAR PRIMARY KEY NOT NULL,
		    user_id VARCHAR REFERENCES users(user_id) ON DELETE CASCADE NOT NULL,
		    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			status VARCHAR NOT NULL DEFAULT 'NEW',
			accrual INTEGER DEFAULT NULL
		);
`

	_, err := db.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("createIfNotExists: %w", err)
	}
	return nil
}

func (s *Storage) RegisterUser(ctx context.Context, username, password string) (string, error) {
	usernameUnique, err := s.isUsernameUnique(ctx, username)
	if err != nil {
		return "", fmt.Errorf("register: user register error: %w", err)
	}
	if !usernameUnique {
		return "", ErrUsernameNotUnique
	}

	userID := auth.GenerateUserID()
	userIDUnique, err := s.isUserIDUnique(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("register: user register error: %w", err)
	}
	for !userIDUnique {
		userIDUnique, err = s.isUserIDUnique(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("register: user register error: %w", err)
		}
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("register: user register error: %w", err)
	}

	query := "INSERT INTO users (user_id, login, password) VALUES ($1,$2,$3)"
	_, err = s.DB.ExecContext(ctx, query, userID, username, hashedPassword)
	if err != nil {
		return "", fmt.Errorf("register: user register error: %w", err)
	}

	return userID, nil
}

func (s *Storage) AuthenticateUser(ctx context.Context, username, password string) (string, error) {
	hashedPassword, err := s.getHashedPasswordByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("authenticateUser: error user auth: %w", err)
	}
	if !auth.IsPasswordEqualsToHashedPassword(password, hashedPassword) {
		return "", fmt.Errorf("authenticateUser: error user auth: %w", ErrUserNotFound)
	}
	userID, err := s.getUserIDByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("authenticateUser: error user auth: %w", err)
	}
	return userID, nil
}

func (s *Storage) getHashedPasswordByUsername(ctx context.Context, username string) (string, error) {
	query := "SELECT password FROM users WHERE login=$1"
	row := s.DB.QueryRowContext(ctx, query, username)

	var hashedPassword string
	err := row.Scan(&hashedPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("getHashedPasswordByUsername: username not found: %w", ErrUserNotFound)
	} else if err != nil {
		return "", fmt.Errorf("getHashedPasswordByUsername: error scanning row: %w", err)
	}
	return hashedPassword, nil
}

func (s *Storage) isUsernameUnique(ctx context.Context, username string) (bool, error) {
	query := "SELECT COUNT(*) FROM users WHERE login=$1"
	row := s.DB.QueryRowContext(ctx, query, username)

	var count int
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("isUsernameUnique: error scanning row: %w", err)
	}
	return count == 0, nil
}

func (s *Storage) isUserIDUnique(ctx context.Context, userID string) (bool, error) {
	query := "SELECT COUNT(*) FROM users WHERE user_id=$1"
	row := s.DB.QueryRowContext(ctx, query, userID)

	var count int
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("isUserIDUnique: error scanning row: %w", err)
	}
	return count == 0, nil
}

func (s *Storage) getUserIDByUsername(ctx context.Context, username string) (string, error) {
	query := "SELECT user_id FROM users WHERE login=$1"
	row := s.DB.QueryRowContext(ctx, query, username)

	var userID string
	err := row.Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("getUserIDByUsername: username not found: %w", ErrUserNotFound)
	} else if err != nil {
		return "", fmt.Errorf("getUserIDByUsername: error scanning row: %w", err)
	}
	return userID, nil
}

func (s *Storage) AddOrder(ctx context.Context, order models.APIAddOrderRequest) error {
	query := "INSERT INTO orders (order_id, user_id) VALUES ($1, $2)"
	_, err := s.DB.ExecContext(ctx, query, order.OrderNumber, order.UserID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				userID, err := s.getUserID(ctx, order.OrderNumber)
				if err != nil {
					return fmt.Errorf("addOrder: %w", err)
				}

				if userID == order.UserID {
					return fmt.Errorf("addOrder: error adding order number: %w", ErrOrderNumberWasAlreadyAddedByThisUser)
				} else {
					return fmt.Errorf("addOrder: error adding order number: %w", ErrOrderNumberWasAlreadyAddedByAnotherUser)
				}
			}
		}
		return fmt.Errorf("addOrder: error adding order number: %w", err)
	}
	return nil
}

func (s *Storage) GetOrders(ctx context.Context, userID string) ([]models.APIGetOrderResponse, error) {
	return nil, nil
}

func (s *Storage) getUserID(ctx context.Context, orderID string) (string, error) {
	query := "SELECT user_id FROM orders WHERE order_id = $1"
	row := s.DB.QueryRowContext(ctx, query, orderID)
	var userID string
	err := row.Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("getUserID: error getting userID by orderID: %w", err)
	}
	return userID, nil
}
