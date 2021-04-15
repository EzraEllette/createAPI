package main

import (
	"autoLambda/gatewayCreator"
	"autoLambda/handleErrors"
	"autoLambda/lambdaCreator"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fmt.Println("Loading configuration...")
	cfg, err := config.LoadDefaultConfig(context.TODO(), func(o *config.LoadOptions) error {
		o.Region = "us-east-1"
		return nil
	})
	check(err)

	fmt.Println("configuration loaded...")

	// _, err = apigatewayRole.Create(cfg)
	// check(err)

	lamClient, err := lambdaCreator.DeployLambdas(cfg)
	handleErrors.Check(err)

	url, err := gatewayCreator.CreateApigateway(cfg, lamClient)
	handleErrors.Check(err)

	fmt.Println(url)
}
