package apigatewayRole

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

// Create creates the necessary policies and role for lambda execution
func Create(cfg aws.Config) (apigatewayARN *string, err error) {

	IAMclient := iam.NewFromConfig(cfg)

	existingRole, err := IAMclient.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: aws.String("autoLambdaGatewayRole"),
	})
	if err == nil {
		fmt.Println("autoLambdaGatewayRole already exists...")
		return existingRole.Role.Arn, nil
	}

	gatewayRolePolicyInput := &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Sid": "",
					"Effect": "Allow",
					"Principal": {
						"Service": "apigateway.amazonaws.com"
					},
					"Action": "sts:AssumeRole"
				}
			]
		}`),
		RoleName: aws.String("autoLambdaGatewayRole"),
	}

	policy := &iam.CreatePolicyInput{
		PolicyName: aws.String("autoLambdaGatewayPolicy"),
		PolicyDocument: aws.String(`{
			"Version": "2012-10-17",
			"Statement": [
					{
							"Effect": "Allow",
							"Action": "lambda:InvokeFunction",
							"Resource": "*"
					}
			]
		}`),
	}

	role, err := IAMclient.CreateRole(context.TODO(), gatewayRolePolicyInput)
	if err != nil {
		return aws.String(""), err
	}
	fmt.Println(role.Role.Arn, role.Role.RoleName)

	createdPolicy, err := IAMclient.CreatePolicy(context.TODO(), policy)
	if err != nil {
		return aws.String(""), err
	}

	_, err = IAMclient.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: createdPolicy.Policy.Arn,
		RoleName:  aws.String("autoLambdaGatewayRole"),
	})
	if err != nil {
		return aws.String(""), err
	}

	fmt.Println("Gateway Lambda Execution Role Successfully Created")
	// this will eventually save us 5 seconds ;)
	time.Sleep(5 * time.Second)
	return role.Role.Arn, nil
}
