package usecase_test

import (
	"fmt"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBcryptCostValues(t *testing.T) {
	fmt.Printf("MinCost: %d\n", bcrypt.MinCost)
	fmt.Printf("DefaultCost: %d\n", bcrypt.DefaultCost)
	fmt.Printf("MaxCost: %d\n", bcrypt.MaxCost)
}

func BenchmarkBcrypt(b *testing.B) {
	password := []byte("secret_password_123")

	costs := []int{bcrypt.MinCost, 6, 8, 10, 12}

	for _, cost := range costs {
		b.Run(fmt.Sprintf("Cost-%d", cost), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := bcrypt.GenerateFromPassword(password, cost)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBcryptCompare(b *testing.B) {
	password := []byte("secret_password_123")
	costs := []int{bcrypt.MinCost, 6, 8, 10}

	for _, cost := range costs {
		hash, _ := bcrypt.GenerateFromPassword(password, cost)
		b.Run(fmt.Sprintf("Compare-Cost-%d", cost), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := bcrypt.CompareHashAndPassword(hash, password)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
