package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
)

func sortUsers(users []User, orderField string, orderBy int) []User {
	switch orderBy {
	case OrderByAsc:
		switch orderField {
		case "Name":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Name > users[j].Name
			})
		case "Id":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Id > users[j].Id
			})
		case "Age":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Age > users[j].Age
			})
		}
	case OrderByDesc:
		switch orderField {
		case "Name":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Name < users[j].Name
			})
		case "Id":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Id < users[j].Id
			})
		case "Age":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Age < users[j].Age
			})
		}
	}
	return users
}

type XMLUser struct {
	ID     		int 	`xml:"id,attr"`
	FirstName   string 	`xml:"first_name"`
	LastName   	string 	`xml:"last_name"`
	Age    		int		`xml:"age"`
	About  		string 	`xml:"about"`
	Gender 		string 	`xml:"gender"`
}

func getUsers(filePath string) ([]User, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(file)
	input := bytes.NewReader(content)
	decoder := xml.NewDecoder(input)

	var users []User
	var user XMLUser
	for {
		tok, tokenErr := decoder.Token()
		if tokenErr != nil && tokenErr != io.EOF {
			fmt.Println("error happend", tokenErr)
			break
		} else if tokenErr == io.EOF {
			break
		}
		if tok == nil {
			fmt.Println("tok is nil break")
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "row" {
				if err := decoder.DecodeElement(&user, &tok); err != nil {
					fmt.Println("error happend", err)
				}
				users = append(users, User{
					Id:     user.ID,
					Name:   user.FirstName + " " + user.LastName,
					Age:    user.Age,
					About:  user.About,
					Gender: user.Gender,
				})
			}
		}
	}
	return users, nil
}
