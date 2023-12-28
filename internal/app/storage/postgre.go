package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vancho-go/gophermart/internal/app/auth"
	"github.com/vancho-go/gophermart/internal/app/logger"
	"github.com/vancho-go/gophermart/internal/app/models"
	"go.uber.org/zap"
	"io"
	"net/http"
	url2 "net/url"
	"runtime"
	"sync"
	"time"
)

var (
	ErrUsernameNotUnique                       = errors.New("username is already in use")
	ErrUserNotFound                            = errors.New("user not found")
	ErrOrderNumberWasAlreadyAddedByThisUser    = errors.New("order number has already been added by this user")
	ErrOrderNumberWasAlreadyAddedByAnotherUser = errors.New("order number has already been added by another user")
	ErrNotEnoughBonuses                        = errors.New("not enough bonuses to use for order")
	ErrEmptyWithdrawalHistory                  = errors.New("no withdrawals for this user")
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
			accrual NUMERIC(20, 2) DEFAULT NULL
		);
		CREATE TABLE IF NOT EXISTS balances (
			user_id VARCHAR REFERENCES users(user_id) ON DELETE CASCADE NOT NULL,
			current NUMERIC(20, 2) DEFAULT 0.0 CHECK (current >=0)
		);
		CREATE TABLE IF NOT EXISTS withdrawals (
		    user_id VARCHAR REFERENCES users(user_id) ON DELETE CASCADE NOT NULL,
		    order_id VARCHAR NOT NULL,
		    sum NUMERIC(20, 2) NOT NULL CHECK (sum >=0),
		    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		    UNIQUE(order_id)
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

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("registerUser: transaction error: %w", err)
		return "", err
	}
	defer tx.Rollback()

	query := "INSERT INTO users (user_id, login, password) VALUES ($1,$2,$3)"
	_, err = tx.ExecContext(ctx, query, userID, username, hashedPassword)
	if err != nil {
		return "", fmt.Errorf("register: user register error: %w", err)
	}

	query = "INSERT INTO balances (user_id) VALUES ($1)"
	_, err = tx.ExecContext(ctx, query, userID)
	if err != nil {
		return "", fmt.Errorf("register: error adding balance wallet: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		err = fmt.Errorf("register: error committing transaction: %w", err)
		return "", err
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
	query := "SELECT order_id,uploaded_at,status,accrual FROM orders WHERE user_id=$1 ORDER BY uploaded_at"

	rows, err := s.DB.QueryContext(ctx, query, userID)

	if rows.Err() != nil {
		return []models.APIGetOrderResponse{}, fmt.Errorf("getOrders: error getting orders: %w", rows.Err())
	}
	defer rows.Close()

	if err != nil {
		return nil, fmt.Errorf("getOrders: error getting orders: %w", err)
	}

	var orderList []models.APIGetOrderResponse
	for rows.Next() {
		var order models.APIGetOrderResponse
		err := rows.Scan(&order.Number, &order.UploadedAt, &order.Status, &order.Accrual)
		if err != nil {
			return nil, fmt.Errorf("getOrders: error getting orders: %w", err)
		}
		orderList = append(orderList, order)
	}

	return orderList, nil
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

func (s *Storage) GetCurrentBonusesAmount(ctx context.Context, userID string) (models.APIGetBonusesAmountResponse, error) {
	var bonusesResponse models.APIGetBonusesAmountResponse

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("getCurrentBonusesAmount: transaction error: %w", err)
		return models.APIGetBonusesAmountResponse{}, err
	}
	defer tx.Rollback()

	query := "SELECT current FROM balances WHERE user_id=$1"
	rowCurrent := tx.QueryRowContext(ctx, query, userID)
	err = rowCurrent.Scan(&bonusesResponse.Current)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			bonusesResponse.Current = 0
		} else {
			err = fmt.Errorf("getCurrentBonusesAmount: error scanning current amount: %w", err)
			return models.APIGetBonusesAmountResponse{}, err
		}
	}

	query = "SELECT COALESCE(SUM(sum),0.0)::float as sum FROM withdrawals WHERE user_id=$1"
	rowSum := tx.QueryRowContext(ctx, query, userID)
	err = rowSum.Scan(&bonusesResponse.Withdrawn)
	if err != nil {
		err = fmt.Errorf("getCurrentBonusesAmount: error scanning withdrawn amount: %w", err)
		return models.APIGetBonusesAmountResponse{}, err
	}

	err = tx.Commit()
	if err != nil {
		err = fmt.Errorf("getCurrentBonusesAmount: error committing transaction: %w", err)
		return models.APIGetBonusesAmountResponse{}, err
	}
	return bonusesResponse, nil
}

func (s *Storage) UseBonuses(ctx context.Context, request models.APIUseBonusesRequest, userID string) (err error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("useBonuses: transaction error: %w", err)
		return err
	}
	defer tx.Rollback()

	var current float64
	query := "SELECT current FROM balances where user_id=$1"
	rowSum := tx.QueryRowContext(ctx, query, userID)
	err = rowSum.Scan(&current)
	if err != nil {
		err = fmt.Errorf("useBonuses: error getting current bonuses amount: %w", err)
		return err
	}

	dif := current - request.Sum

	if dif < 0 {
		return fmt.Errorf("useBonuses: %w", ErrNotEnoughBonuses)
	}

	query = "UPDATE balances SET current=$1 WHERE user_id=$2"
	_, err = tx.ExecContext(ctx, query, dif, userID)
	if err != nil {
		err = fmt.Errorf("useBonuses: error updating current bonuses amount: %w", err)
		return err
	}

	query = "INSERT INTO withdrawals (user_id,order_id,sum) VALUES ($1,$2,$3)"
	_, err = tx.ExecContext(ctx, query, userID, request.OrderNumber, request.Sum)
	if err != nil {
		err = fmt.Errorf("useBonuses: error inserting data to withdrawals: %w", err)
		return err
	}
	err = tx.Commit()
	if err != nil {
		err = fmt.Errorf("useBonuses: error committing transaction: %w", err)
		return err
	}
	return nil
}

func (s *Storage) GetWithdrawalsHistory(ctx context.Context, userID string) ([]models.APIGetWithdrawalsHistoryResponse, error) {
	query := "SELECT order_id,sum,processed_at FROM withdrawals WHERE user_id=$1 ORDER BY processed_at"

	rows, err := s.DB.QueryContext(ctx, query, userID)
	if rows.Err() != nil {
		return []models.APIGetWithdrawalsHistoryResponse{}, fmt.Errorf("getWithdrawalsHistory: error getting orders: %w", rows.Err())
	}
	defer rows.Close()

	if err != nil {
		return nil, fmt.Errorf("getWithdrawalsHistory: error getting withdrawal history: %w", err)
	}

	var withdrawalsHistory []models.APIGetWithdrawalsHistoryResponse
	for rows.Next() {
		var withdrawalHistory models.APIGetWithdrawalsHistoryResponse
		err = rows.Scan(&withdrawalHistory.Order, &withdrawalHistory.Sum, &withdrawalHistory.ProcessedAt)
		if err != nil {
			return nil, fmt.Errorf("getWithdrawalsHistory: error getting orders: %w", err)
		}
		withdrawalsHistory = append(withdrawalsHistory, withdrawalHistory)
	}

	if len(withdrawalsHistory) == 0 {
		return withdrawalsHistory, fmt.Errorf("getWithdrawalsHistory: %w", ErrEmptyWithdrawalHistory)
	}

	return withdrawalsHistory, nil

}

func (s *Storage) HandleOrderNumbers(ctx context.Context, accrualSystemAddress string, logger logger.Logger) {
	// Отсюда будут запускаться задачи на обновление статуса заказа

	select {
	case <-ctx.Done():
		logger.Info("handleOrderNumbers: update task cancelled by context")
	default:
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		orderNumbersChannel, err := s.getNotCalculatedOrderNumbers(ctx, logger)
		if err != nil {
			logger.Error("handleOrderNumbers:", zap.Error(err))
			return
		}

		var stageUpdateOrderStatusChannels []<-chan string
		var updateErrors []<-chan error

		for i := 0; i < runtime.NumCPU(); i++ {
			updateOrderStatusChannel, updateOrderStatusErrors, err := s.prepareAndUpdateOrderStatus(ctx, orderNumbersChannel, accrualSystemAddress)
			if err != nil {
				logger.Error("handleOrderNumbers:", zap.Error(err))
				return
			}
			stageUpdateOrderStatusChannels = append(stageUpdateOrderStatusChannels, updateOrderStatusChannel)
			updateErrors = append(updateErrors, updateOrderStatusErrors)
		}
		stageUpdateOrderStatusMerged := mergeChannels(ctx, stageUpdateOrderStatusChannels...)
		errorsMerged := mergeChannels(ctx, updateErrors...)

		orderStatusConsumer(ctx, stageUpdateOrderStatusMerged, errorsMerged, logger)
	}

}

func (s *Storage) getNotCalculatedOrderNumbers(ctx context.Context, logger logger.Logger) (<-chan string, error) {
	// producer

	outputChannel := make(chan string)

	query := "SELECT order_id FROM orders WHERE status NOT IN ('INVALID', 'PROCESSED')"
	rows, err := s.DB.Query(query)

	if rows.Err() != nil {
		logger.Error("getNotCalculatedOrderNumbers:", zap.Error(err))
		//todo
	}

	if err != nil {
		logger.Error("getNotCalculatedOrderNumbers:", zap.Error(err))
		//todo
	}
	go func() {
		defer close(outputChannel)
		for rows.Next() {
			var orderNumber string
			if err := rows.Scan(&orderNumber); err != nil {
				//todo
				logger.Error("getNotCalculatedOrderNumbers:", zap.Error(err))
			}
			select {
			case <-ctx.Done():
				return
			case outputChannel <- orderNumber:
			}
		}
	}()

	return outputChannel, nil
}

func (s *Storage) prepareAndUpdateOrderStatus(ctx context.Context, orderNumbers <-chan string, accrualSystemAddress string) (<-chan string, <-chan error, error) {
	outChannel := make(chan string)
	errorChannel := make(chan error)

	go func() {
		defer close(outChannel)
		defer close(errorChannel)

		select {
		case <-ctx.Done():
			return
		case orderNumber, ok := <-orderNumbers:
			if ok {
				ctxWTO, cancel := context.WithTimeout(ctx, time.Second*5)
				defer cancel()

				err := s.updateOrderStatus(ctxWTO, orderNumber, accrualSystemAddress)
				if err != nil {
					errorChannel <- err
				} else {
					outChannel <- fmt.Sprintf("prepareAndUpdateOrderStatus: order '%s' updated", orderNumber)
				}
			} else {
				return
			}
		}
	}()
	return outChannel, errorChannel, nil
}

func (s *Storage) updateOrderStatus(ctx context.Context, orderNumber string, accrualSystemAddress string) error {
	orderInfo, err := getOrderInfo(ctx, orderNumber, accrualSystemAddress)
	if err != nil {
		return fmt.Errorf("updateOrderStatus: error getting order info: %w", err)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		err = fmt.Errorf("updateOrderStatus: error beginning transaction: %w", err)
		return err
	}
	defer tx.Rollback()

	query := "UPDATE orders SET status = $1, accrual = $2 WHERE order_id = $3"
	_, err = tx.ExecContext(ctx, query, orderInfo.Status, orderInfo.Accrual, orderNumber)
	if err != nil {
		return fmt.Errorf("updateOrderStatus: error updating status for order %s: %w", orderNumber, err)
	}
	if orderInfo.Accrual > 0 {
		query = "UPDATE balances SET current = current + $1 WHERE user_id = (SELECT user_id FROM orders WHERE order_id = $2) RETURNING current"
		_, err = tx.ExecContext(ctx, query, orderInfo.Accrual, orderNumber)
		if err != nil {
			return fmt.Errorf("updateOrderStatus: error updating balance for order %s: %w", orderNumber, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		err = fmt.Errorf("updateOrderStatus: error committing transaction: %w", err)
		return err
	}

	return nil
}

func getOrderInfo(ctx context.Context, orderNumber string, accrualSystemAddress string) (*models.APIOrderInfoResponse, error) {
	url, err := url2.JoinPath(accrualSystemAddress, "/api/orders/", orderNumber)
	if err != nil {
		return nil, fmt.Errorf("getOrderInfo: error joining path: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("getOrderInfo: error with request: %w", err)
	}

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getOrderInfo: error get: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var orderInfo models.APIOrderInfoResponse
		if err := json.NewDecoder(resp.Body).Decode(&orderInfo); err != nil {
			return nil, fmt.Errorf("getOrderInfo: error decoding JSON resp: %w", err)
		}
		return &orderInfo, nil
	case http.StatusNoContent:
		return nil, fmt.Errorf("getOrderInfo: order %s not registered in the system", orderNumber)
	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("getOrderInfo: rate limit exceeded, retry after %s seconds", retryAfter)
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("getOrderInfo: interna; server error")
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getOrderInfo: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

}

func mergeChannels[T any](ctx context.Context, ce ...<-chan T) <-chan T {
	var wg sync.WaitGroup
	out := make(chan T)

	output := func(c <-chan T) {
		defer wg.Done()
		for n := range c {
			select {
			case out <- n:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(ce))
	for _, c := range ce {
		go output(c)

	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

func orderStatusConsumer(ctx context.Context, orderInfoResult <-chan string, orderInfoErrors <-chan error, logger logger.Logger) {
	for {
		select {
		case <-ctx.Done():
			logger.Error("orderStatusConsumer:", zap.Error(ctx.Err()))
			return
		case err, ok := <-orderInfoErrors:
			if ok {
				//todo
				logger.Error("orderStatusConsumer:", zap.Error(err))
			}

		case order, ok := <-orderInfoResult:
			if ok {
				//todo
				logger.Info("orderStatusConsumer:" + order)
			} else {
				return
			}

		}

	}
}
