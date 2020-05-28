package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

var validAccessToken = "access"

func SearchServer(w http.ResponseWriter, r *http.Request) {
	accessToken := r.Header.Get("AccessToken")
	if accessToken != validAccessToken {
		http.Error(w, "Bad AccessToken", http.StatusUnauthorized)
		return
	}

	query := r.FormValue("query")

	if query == "timeout_error" {
		time.Sleep(time.Second * 3)
	}

	if query == "invalid_json" {
		w.Write([]byte("invalid json test"))
		return
	}

	if query == "bad_request_invalid_json" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid json test"))
		return
	}

	if query == "bad_request_unknown_error" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SearchErrorResponse{"UnknownError"})
		return
	}

	if query == "internal_server_error" {
		http.Error(w, "InternalServerError", http.StatusInternalServerError)
		return
	}

	limit, err := strconv.Atoi(r.FormValue("limit"))
	if err != nil {
		http.Error(w, "InternalServerError", http.StatusInternalServerError)
		return
	}

	offset, err := strconv.Atoi(r.FormValue("offset"))
	if err != nil {
		http.Error(w, "InternalServerError", http.StatusInternalServerError)
		return
	}

	orderField := r.FormValue("order_field")
	if orderField == "" {
		orderField = "Name"
	}
	orderBy, err := strconv.Atoi(r.FormValue("order_by"))
	if err != nil {
		http.Error(w, "InternalServerError", http.StatusInternalServerError)
		return
	}

	switch orderField {
	case "Name", "Id", "Age":
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SearchErrorResponse{"ErrorBadOrderField"})
		return
	}

	allUsers, err := getUsers("dataset.xml")
	if err != nil {
		http.Error(w, "InternalServerError", http.StatusInternalServerError)
		return
	}
	var users []User
	if query != "" {
		for _, user := range allUsers {
			if strings.Contains(user.Name, query) || strings.Contains(user.About, query) {
				users = append(users, user)
			}
		}
	} else {
		users = allUsers
	}

	if orderBy > 1 || orderBy < -1 {
		http.Error(w, "ErrorBadOrderBy", http.StatusBadRequest)
		return
	}

	users = sortUsers(users, orderField, orderBy)

	if len(users) > limit {
		users = users[:limit]
	}

	if len(users) > offset {
		users = users[offset:]
	}

	json.NewEncoder(w).Encode(users)
}

func TestSearching(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	sc := SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}

	sReq := SearchRequest{Query: "Boyd", Limit: 25}
	expUser := User{
			Id:     0,
			Name:   "Boyd Wolf",
			Age:    22,
			About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
			Gender: "male",
	}
	expLength := 1

	result, err := sc.FindUsers(sReq)
	if err != nil {
		t.Errorf("unexpected error: %#v", err)
	}
	if len(result.Users) != expLength {
		t.Errorf("unexpected users length, expected %v, got %v", expLength, len(result.Users))
	}
	if !reflect.DeepEqual(expUser, result.Users[0]) {
		t.Errorf("wrong user, expected %#v, got %#v", expUser, result.Users[0])
	}
}

func TestLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	sc := SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}

	cases := []struct{
		sRequest 	SearchRequest
		expLength 	int
	}{
		{SearchRequest{Limit: 30}, 25},
		{SearchRequest{Limit: 10}, 10},
	}

	for caseNum, item := range cases {
		result, err := sc.FindUsers(item.sRequest)
		if err != nil {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if len(result.Users) != item.expLength {
			t.Errorf("[%d] unexpected users length, expected %v, got %v", caseNum,item.expLength, len(result.Users))
		}
	}
}

type TestCase struct {
	Request 	SearchRequest
	ErrorStr 	string
}

func TestErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	sc := SearchClient{
		AccessToken: validAccessToken,
		URL:         ts.URL,
	}

	cases := []TestCase{
		{SearchRequest{Offset: -1}, fmt.Sprintf("offset must be > 0")},
		{SearchRequest{Limit:-1},fmt.Sprintf("limit must be > 0")},
		{SearchRequest{OrderField: "Test"}, fmt.Sprintf("OrderFeld %s invalid", "Test")},
		{SearchRequest{Query: "timeout_error"}, fmt.Sprintf("timeout for limit=1&offset=0&order_by=0&order_field=&query=timeout_error")},
		{SearchRequest{Query: "bad_request_unknown_error"}, fmt.Sprintf("unknown bad request error: %s", "UnknownError")},
		{SearchRequest{Query: "bad_request_invalid_json"}, fmt.Sprintf("cant unpack error json: %s", "invalid character 'i' looking for beginning of value")},
		{SearchRequest{Query: "invalid_json"}, fmt.Sprintf("cant unpack result json: %s",  "invalid character 'i' looking for beginning of value")},
		{SearchRequest{Query: "internal_server_error"}, fmt.Sprintf("SearchServer fatal error")},
	}

	for caseNum, item := range cases {
		_, err := sc.FindUsers(item.Request)
		if err == nil {
			t.Errorf("[%d] expected error, got nil", caseNum)
		}
		if err.Error() != item.ErrorStr {
			t.Errorf("[%d] got error: %#v, want: %#v", caseNum, err.Error(), item.ErrorStr)
		}
	}
}

func TestBadTokenError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))

	scBad := SearchClient{
		AccessToken: "",
		URL:         ts.URL,
	}
	tokenErr := "Bad AccessToken"
	_, err := scBad.FindUsers(SearchRequest{})
	if err == nil {
		t.Errorf("err must not be nil")
	}
	if err.Error() != tokenErr {
		t.Errorf("error want: %v, got %v", tokenErr, err.Error())
	}
}

func TestClientUnknownError(t *testing.T) {
	scBad := SearchClient{
		AccessToken: validAccessToken,
		URL:         "",
	}

	unError := fmt.Sprintf("unknown error %s", "Get \"?limit=1&offset=0&order_by=0&order_field=&query=\": unsupported protocol scheme \"\"")
	_, err := scBad.FindUsers(SearchRequest{})
	if err == nil {
		t.Errorf("err must not be nil")
	}
	if err.Error() != unError {
		t.Errorf("error want: %v, got %v", unError, err.Error())
	}
}