package handleErrors

/*
- able to log errors
- able to check if there is an Error
- able to stop the program if there is an error
- able to keep the program going if there is an error that needs to be handled
*/

func Check(err error) {
	if err != nil {
		panic(err)
	}
}

func HandleError(err error, callback func(err error)) {
	if err != nil {
		callback(err)
	}
}
