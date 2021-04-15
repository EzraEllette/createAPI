package lambdaRole

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// Create creates the necessary policies and role for lambda execution
func Create(cfg aws.Config) (lamARN *string, err error) {

	IAMclient := iam.NewFromConfig(cfg)

	existingRole, err := IAMclient.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: aws.String("autoLambdaExecutionRole"),
	})
	if err == nil {
		fmt.Println("autoLambdaExecutionRole already exists...")
		return existingRole.Role.Arn, nil
	}

	lambdaRolePolicyInput := &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(`{
				"Version": "2012-10-17",
				"Statement": [
					{
							"Effect": "Allow",
							"Principal": {
								"Service": [
										"lambda.amazonaws.com",
										"apigateway.amazonaws.com"
								]
							},
							"Action": "sts:AssumeRole"
					}
				]
			}
		`),
		RoleName: aws.String("autoLambdaExecutionRole"),
	}

	role, err := IAMclient.CreateRole(context.TODO(), lambdaRolePolicyInput)
	if err != nil {
		return aws.String(""), err
	}
	fmt.Println(*role.Role.Arn, *role.Role.RoleName)

	_, err = IAMclient.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: aws.String("arn:aws:iam::aws:policy/service-role/AWSLambdaRole"),
		RoleName:  aws.String("autoLambdaExecutionRole"),
	})
	if err != nil {
		return aws.String(""), err
	}

	// this will eventually save us 5 seconds ;)
	time.Sleep(5 * time.Second)
	time.Sleep(4 * time.Second)
	fmt.Println("Lambda Execution Role Successfully Created")
	return role.Role.Arn, nil
}
