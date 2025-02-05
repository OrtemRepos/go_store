package luhn

import "strconv"

func CheckValidNumber(number string) bool {
	sum := 0
	parity := len(number) % 2
	for pos, char := range number {
		digit, err := strconv.Atoi(string(char))
		if err != nil {
			return false
		}
		if pos % 2 == parity {
			digit *= 2
			if digit > 9 {
				digit = digit - 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}