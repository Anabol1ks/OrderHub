package hashing

import "golang.org/x/crypto/bcrypt"

type Bcrypt struct {
	cost int
}

func NewBcrypt(cost int) *Bcrypt {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &Bcrypt{cost: cost}
}

func (b *Bcrypt) Hash(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	return string(h), err
}

func (b *Bcrypt) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
