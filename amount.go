package paymentmanager

import (
	"webtyp.com/fmt"
)

func FormatAmount(amount int64, currency string) string {
	if currency == CurrencyCLP {
		s := fmt.Sprint(amount)
		res := make([]byte, 0, len(s)+len(s)/3+1)
		res = append(res, '$')
		for i := 0; i < len(s); i++ {
			if i > 0 && (len(s)-i)%3 == 0 {
				res = append(res, '.')
			}
			res = append(res, s[i])
		}
		return string(res)
	}

	if currency == CurrencyUSD {
		s := fmt.Sprint(amount)
		for len(s) < 3 {
			s = "0" + s
		}
		intPart := s[:len(s)-2]
		decPart := s[len(s)-2:]
		return "US$" + intPart + "," + decPart
	}

	return fmt.Sprintf("%d %s", amount, currency)
}
