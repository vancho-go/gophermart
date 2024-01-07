package handlers

import (
	"errors"
	"strconv"
	"strings"
)

// isOrderNumberValid проверяет номер заказа с использованием алгоритма Луна.
// Возвращает true если номер валидный, иначе false и соответствующую ошибку.
func isOrderNumberValid(orderNumber string) error {
	// Удаляем все пробелы для чистоты ввода
	cleanOrderNumber := strings.ReplaceAll(orderNumber, " ", "")
	if cleanOrderNumber == "" {
		return errors.New("isOrderNumberValid: order number is empty")
	}

	// Алгоритм Луна:
	sum := 0
	length := len(cleanOrderNumber)
	for i := length - 1; i >= 0; i-- {
		n, err := strconv.Atoi(string(cleanOrderNumber[i]))
		if err != nil {
			return errors.New("isOrderNumberValid: order number contains invalid characters")
		}

		// Удваиваем каждую вторую цифру, начиная с конца
		if (length-i-1)%2 == 1 {
			n *= 2
			if n > 9 {
				n -= 9 // Если результат удвоения больше 9, вычитаем 9
			}
		}

		sum += n
	}

	if sum%10 != 0 {
		return errors.New("isOrderNumberValid: order number contains invalid characters")
	}
	// Если сумма кратна 10, номер валидный
	return nil
}
