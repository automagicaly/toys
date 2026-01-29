package entities

import "time"

const Missing = 0

const (
	AlfaVersion VersionNumber = iota
	Latest      VersionNumber = iota - 1
)

type ID uint64
type VersionNumber uint64
type Optional[T any] *T

type CommonFields struct {
	Version VersionNumber
	ID      ID
}

type Tenant struct {
	CommonFields
}

type Entity struct {
	CommonFields
	Tenant *Tenant
}

type Class struct {
	Entity
	Students []Student
}

type UserType uint8

const (
	StudentUserType UserType = iota
	ParentUserType
	StaffUserType
	FamilyMemberUserType
)

type User struct {
	Entity
	Type UserType
	Name string
}

func (u *User) GetIDs() []ID {
	return []ID{u.ID}
}

func (u *User) Matches(id ID) bool {
	return u.ID == id
}

type Everyone struct {
}

func (e *Everyone) GetIDs() []ID {
	return []ID{}
}

func (e *Everyone) Matches(id ID) bool {
	return true
}

type Parents struct {
	Entity
	Mother User
	Father Optional[User]
}

type Gender bool
type Student struct {
	User
	Parents     Parents
	DateOfBirth time.Time
	Gender      Gender
}
type Family struct {
	Entity
	Parents  Parents
	Students []Student
	Others   []User
}

type Job uint8

const (
	HelperJob Job = iota
	TeacherJob
	PrincipalJob
	CoordinatorJob
)

type Staff struct {
	User
	Position Job
}
