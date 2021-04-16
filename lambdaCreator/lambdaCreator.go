package lambdaCreator

import (
	"archive/zip"
	"autoLambda/IAMRoles/lambdaRole"
	"autoLambda/eachFile"
	"autoLambda/handleErrors"
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

var LambdaARNs map[string]*string

func GetARN(functionName string) *string {
	return LambdaARNs[functionName]
}

func init() {
	LambdaARNs = map[string]*string{}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func createLambda(client *lambda.Client, lambdaRoleARN *string, functionName *string, zipFile []byte) {
	createFunctionInput := &lambda.CreateFunctionInput{
		Code: &types.FunctionCode{
			ZipFile: zipFile,
		},
		FunctionName: functionName,
		Role:         lambdaRoleARN,
		Handler:      aws.String(*functionName + ".handler"),
		Runtime:      types.RuntimeNodejs14x,
	}
	fmt.Println("creating function...")
	lamFunction, err := client.CreateFunction(context.TODO(), createFunctionInput)
	check(err)
	LambdaARNs[*functionName] = lamFunction.FunctionArn

	fmt.Println("function created", *lamFunction.FunctionArn)
}

// DeployLambdas creates lambdas from the functions folder
func DeployLambdas(cfg aws.Config) (lamClient *lambda.Client, err error) {

	lambdaRoleARN, err := lambdaRole.Create(cfg)
	handleErrors.Check(err)

	lamClient = lambda.NewFromConfig(cfg)
	eachFile.Recursive("functions", func(fileName string, file []byte) {
		var functionName string = toFunctionName(fileName)

		zipFile := compressFile(toFileName(fileName), file)

		createLambda(lamClient, lambdaRoleARN, aws.String(functionName), zipFile)
	})
	return
}

func toFunctionName(fileName string) string {
	file := strings.Split(fileName, "/")
	lastPart := strings.Split(file[len(file)-1], ".")[0]
	file[len(file)-1] = lastPart
	return string(strings.Join(file, "-"))[10:]
}

func getFileExtension(file string) string {
	extension := ""

	for i := len(file) - 1; file[i] != '.'; i-- {
		extension = string(file[i]) + extension
	}

	return "." + extension
}

func toFileName(fileName string) string {
	return toFunctionName(fileName) + getFileExtension(fileName)
}

func compressFile(fileName string, file []byte) []byte {

	buff := new(bytes.Buffer)

	zipWriter := zip.NewWriter(buff)

	f, err := zipWriter.Create(fileName)
	check(err)
	_, err = f.Write(file)
	check(err)
	err = zipWriter.Close()
	check(err)

	return buff.Bytes()
}
