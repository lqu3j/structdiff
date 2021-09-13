package structdiff

import (
	"encoding/json"
	"log"
	"testing"
)

type Student struct {
	Name      string
	Subjects1 []*Subject `comparedby:"Name"`
	Subjects2 []*Subject

	Contacts1 Contacts `comparedby:",direct"`
	Contacts2 Contacts
}

type Subject struct {
	Name  string
	Score float64
}

type Contacts struct {
	Phone string
}

func TestCompare(t *testing.T) {
	diffdetails := &DiffDetails{}

	oldStudent := &Student{
		Name:      "LiLei",
		Subjects1: []*Subject{{Name: "Math", Score: 80}, {Name: "Geography", Score: 80}},
		Subjects2: []*Subject{{Name: "Math", Score: 80}, {Name: "Geography", Score: 80}},
		Contacts1: Contacts{Phone: "12345678"},
		Contacts2: Contacts{Phone: "12345678"},
	}

	newStudent := &Student{
		Name:      "LiLei",
		Subjects1: []*Subject{{Name: "Math", Score: 88}, {Name: "English", Score: 88}},
		Subjects2: []*Subject{{Name: "Math", Score: 88}, {Name: "English", Score: 88}},
		Contacts1: Contacts{Phone: "123456"},
		Contacts2: Contacts{Phone: "123456"},
	}

	diffdetails, err := Diff(newStudent, oldStudent)
	if err != nil {
		panic(err)
	}
	body, _ := json.Marshal(diffdetails)
	log.Println(string(body))
}
