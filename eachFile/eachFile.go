package eachFile

import (
	"io/ioutil"
)

func readFile(path string) (file []byte, err error) {
	file, err = ioutil.ReadFile(path)
	return
}

// Recursive will perform a breadth first search from base path into all directories and execute a callback on any file found
func Recursive(basePath string, callback func(filename string, file []byte)) (err error) {

	folders := make(chan string, 100)
	folders <- basePath
	files := make(chan string, 400)

	for len(folders) > 0 {
		currentFolder := <-folders
		directories, err := ioutil.ReadDir(currentFolder)

		if currentFolder == "." {
			currentFolder = ""
		}

		for _, file := range directories {
			if err != nil {
				return err
			}
			if file.IsDir() {
				folders <- currentFolder + "/" + file.Name()
			} else {
				files <- currentFolder + "/" + file.Name()
			}
		}
	}

	for len(files) > 0 {
		filename := <-files
		file, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		callback(filename, file)
	}
	return
}
