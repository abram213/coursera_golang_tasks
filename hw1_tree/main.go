package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
)

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

func dirTree(output io.Writer, path string, printFiles bool) error {
	return printDir(output, path, printFiles, "")
}

func printDir(output io.Writer, path string, printFiles bool, prePath string) error {
	dirItems, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	var items []os.FileInfo
	for _, item := range dirItems {
		if item.IsDir() {
			items = append(items, item)
		} else if printFiles {
			items = append(items, item)
		}
	}

	for index, item := range items {
		var childPrefix, prefix string
		if index == len(items)-1 {
			prefix, childPrefix = "└───", "\t"
		} else {
			prefix, childPrefix = "├───", "│\t"
		}

		var size string
		if item.Size() == 0 {
			size = "empty"
		} else {
			size = strconv.Itoa(int(item.Size())) + "b"
		}

		if item.IsDir() {
			fmt.Fprintln(output, prePath+prefix+item.Name())
			printDir(output, path+string(os.PathSeparator)+item.Name(), printFiles, prePath+childPrefix)
		} else {
			fmt.Fprintln(output, prePath+prefix+item.Name()+" ("+size+")")
		}
	}

	return nil
}
