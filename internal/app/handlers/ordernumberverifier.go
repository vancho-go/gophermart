package handlers

import (
	"errors"
	"strconv"
	"strings"
)

// isOrderNumberValid проверяет номер заказа с использованием алгоритма Луна.
// Возвращает true если номер валидный, иначе false и соответствующую ошибку.
func isOrderNumberValid(orderNumber string) (bool, error) {
	// Удаляем все пробелы для чистоты ввода
	cleanOrderNumber := strings.ReplaceAll(orderNumber, " ", "")
	if cleanOrderNumber == "" {
		return false, errors.New("isOrderNumberValid: order number is empty")
	}

	// Переводим номер в обратном порядке для удобства итерации
	reversedOrderNumber := reverseString(cleanOrderNumber)

	// Алгоритм Луна:
	sum := 0
	for i, char := range reversedOrderNumber {
		n, err := strconv.Atoi(string(char))
		if err != nil {
			return false, errors.New("isOrderNumberValid: order number contains invalid characters")
		}

		// Удваиваем каждую вторую цифру, начиная с конца
		if i%2 == 1 {
			n *= 2
			if n > 9 {
				n -= 9 // Если результат удвоения больше 9, вычитаем 9
			}
		}

		sum += n
	}

	// Если сумма кратна 10, номер валидный
	return sum%10 == 0, nil
}

// reverseString возвращает строку в обратном порядке.
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
