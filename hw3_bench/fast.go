package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

type User struct {
	Browsers []string 	`json:"browsers"`
	Email string 		`json:"email"`
	Name string 		`json:"name"`
}

var dataPool = sync.Pool{
	New: func() interface{} {
		return User{}
	},
}

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	reader := bufio.NewReader(file)

	var line []byte
	//var seenBrowsers []string
	var seenBrowsers map[string]bool = map[string]bool{}
	var uniqueBrowsers int
	var foundUsers string
	var i = -1

	var user User

	for {
		line, err = reader.ReadBytes('\n')
		if err != nil {
			break
		}

		if err == io.EOF {
			break
		}

		err := user.UnmarshalJSON(line)
		if err != nil {
			panic(err)
		}
		i++

		isAndroid := false
		isMSIE := false

		for _, browser := range user.Browsers {
			if strings.Contains(browser, "Android") {
				isAndroid = true
				//notSeenBefore := true
				if _, ok := seenBrowsers[browser]; !ok {
					seenBrowsers[browser] = true
					uniqueBrowsers++
				}
				/*for _, item := range seenBrowsers {
					if item == browser {
						notSeenBefore = false
					}
				}*/
				/*if notSeenBefore {
					// log.Printf("SLOW New browser: %s, first seen: %s", browser, user["name"])
					seenBrowsers = append(seenBrowsers, browser)
					uniqueBrowsers++
				}*/
			}
			if strings.Contains(browser, "MSIE") {
				isMSIE = true
				if _, ok := seenBrowsers[browser]; !ok {
					seenBrowsers[browser] = true
					uniqueBrowsers++
				}
				/*notSeenBefore := true
				for _, item := range seenBrowsers {
					if item == browser {
						notSeenBefore = false
					}
				}
				if notSeenBefore {
					// log.Printf("SLOW New browser: %s, first seen: %s", browser, user["name"])
					seenBrowsers = append(seenBrowsers, browser)
					uniqueBrowsers++
				}*/
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}

		// log.Println("Android and MSIE user:", user["name"], user["email"])
		email := strings.ReplaceAll(user.Email, "@", " [at] ")
		foundUsers += "["+strconv.Itoa(i)+"] "+ user.Name + " <"+email+">\n"
	}

	if err != io.EOF {
		fmt.Printf(" > Failed!: %v\n", err)
		panic(err.Error())
	}

	fmt.Fprintln(out, "found users:\n"+foundUsers)
	fmt.Fprintln(out, "Total unique browsers", len(seenBrowsers))
}