package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// OwnerKind различает, кому принадлежит domain.Session: зарегистрированному
// пользователю или анонимному гостю.
type OwnerKind string

const (
	OwnerUser  OwnerKind = "user"
	OwnerGuest OwnerKind = "guest"
)

// Owner — стабильный идентификатор владельца сессии прохождения.
//
// Это НЕ session_id из JWT (тот живёт ровно один логин и ротируется при
// рефреше) и НЕ refresh_tokens.session_id. Owner.ID — это либо users.id
// (стабилен на весь срок жизни аккаунта), либо guest_sessions.id
// (стабилен, пока не истёк GuestTTL или пока не вызван ClaimGuest).
type Owner struct {
	Kind OwnerKind
	ID   uuid.UUID
}

func UserOwner(id uuid.UUID) Owner  { return Owner{Kind: OwnerUser, ID: id} }
func GuestOwner(id uuid.UUID) Owner { return Owner{Kind: OwnerGuest, ID: id} }

func (o Owner) IsUser() bool  { return o.Kind == OwnerUser }
func (o Owner) IsGuest() bool { return o.Kind == OwnerGuest }

func (o Owner) Valid() bool {
	return (o.Kind == OwnerUser || o.Kind == OwnerGuest) && o.ID != uuid.Nil
}

func (o Owner) String() string {
	return fmt.Sprintf("%s:%s", o.Kind, o.ID)
}
