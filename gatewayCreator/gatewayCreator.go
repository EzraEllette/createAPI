package gatewayCreator

import (
	"autoLambda/eachFile"
	"autoLambda/handleErrors"
	"autoLambda/lambdaCreator"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// resourceFunctions ResourceId, functionName
var resourceFunctions map[string]string

func init() {
	resourceFunctions = map[string]string{}
}

type resourceTrieNode struct {
	Children map[string]*resourceTrieNode `json:"children"`
	Value    string                       `json:"value"`
}

func (t *resourceTrieNode) insert(value string) {
	currentNode := t

	pathParts := strings.Split(value, "/")

	if strings.Contains(pathParts[len(pathParts)-1], ".") {
		pathParts[len(pathParts)-1] = strings.Split(pathParts[len(pathParts)-1], ".")[0]
	}

	for _, resource := range pathParts {
		if newNode, ok := currentNode.Children[resource]; ok {
			currentNode = newNode
		} else {
			newNode := &resourceTrieNode{
				Children: map[string]*resourceTrieNode{},
				Value:    resource,
			}
			currentNode.Children[resource] = newNode
			currentNode = currentNode.Children[resource]
		}
	}
}

func (t *resourceTrieNode) createResources(client *apigateway.Client, apiId *string, parentId *string) {
	nodes := make(chan *resourceTrieNode, 400)
	// children := make(chan *resourceTrieNode, 400)

	nodes <- t

	for len(nodes) > 0 {
		currentNode := <-nodes
		for k, v := range currentNode.Children {
			resourceConfig := &apigateway.CreateResourceInput{
				RestApiId: apiId,
				PathPart:  &k,
				ParentId:  parentId,
			}
			resource, err := client.CreateResource(context.TODO(), resourceConfig)

			if err != nil {
				panic(err)
			}

			methodInput := &apigateway.PutMethodInput{
				AuthorizationType: aws.String("NONE"),
				HttpMethod:        aws.String("ANY"),
				RestApiId:         apiId,
				ResourceId:        resource.Id,
				ApiKeyRequired:    false,
			}

			_, err = client.PutMethod(context.TODO(), methodInput)
			if err != nil {
				panic(err)
			}

			responseOptions := &apigateway.PutMethodResponseInput{
				RestApiId:      apiId,
				StatusCode:     aws.String("200"),
				ResourceId:     resource.Id,
				HttpMethod:     aws.String("ANY"),
				ResponseModels: map[string]string{"application/json": "Empty"},
			}

			_, err = client.PutMethodResponse(context.TODO(), responseOptions)
			handleErrors.Check(err)

			uri := "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/" + *lambdaCreator.GetARN(k) + "/invocations"
			fmt.Println(uri)
			if len(v.Children) == 0 {
				fmt.Println("adding integration")
				integrationConfig := &apigateway.PutIntegrationInput{
					HttpMethod:            aws.String("ANY"),
					ResourceId:            resource.Id,
					RestApiId:             apiId,
					Type:                  types.IntegrationTypeAws,
					IntegrationHttpMethod: aws.String("POST"),
					Uri:                   &uri,
				}

				_, err = client.PutIntegration(context.TODO(), integrationConfig)
				if err != nil {
					panic(err)
				}
				resourceFunctions[*resource.Id] = *lambdaCreator.GetARN(k)
				integrationResponseOptions := &apigateway.PutIntegrationResponseInput{
					HttpMethod:        aws.String("ANY"),
					ResourceId:        resource.Id,
					RestApiId:         apiId,
					StatusCode:        aws.String("200"),
					ResponseTemplates: map[string]string{"application/json": ""},
					// ResponseParameters: map[string]string{
					// 	"Access-Control-Allow-Origin":  "*",
					// 	"Access-Control-Allow-Headers": "'Content-Type,X-Amz-Date,Authorization,x-api-key,x-amz-security-token'",
					// 	"Access-Control-Allow-Methods": "'GET,POST,DELETE,PUT,PATCH,OPTIONS'",
					// },
				}

				_, err := client.PutIntegrationResponse(context.TODO(), integrationResponseOptions)
				if err != nil {
					panic(err)
				}
			}

			v.createResources(client, apiId, resource.Id)
		}
	}
}

func new(basePath string) *resourceTrieNode {
	trie := &resourceTrieNode{
		Children: map[string]*resourceTrieNode{},
		Value:    "/",
	}
	eachFile.Recursive(basePath, func(filename string, file []byte) {
		if filename == basePath {
			return
		}
		trie.insert(filename[10:])
	})
	return trie
}

func CreateApigateway(cfg aws.Config, lamClient *lambda.Client) (stageURL string, err error) {
	gatewayClient := apigateway.NewFromConfig(cfg)

	APIInput := &apigateway.CreateRestApiInput{
		Name: aws.String("autoLambdaTestAPI"),
		Policy: aws.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": "*",
					"Action": [
						"execute-api:Invoke",
						"execute-api:InvalidateCache"
					],
					"Resource": [
						"arn:aws:execute-api:*:*:*/test/*"
					]
				}
			]
		} `),
	}

	// 	{
	//     "Version": "2012-10-17",
	//     "Statement": [
	//         {
	//             "Effect": "Allow",
	//             "Principal": {
	//                 "AWS": "*"
	//             },
	//             "Action": "execute-api:Invoke",
	//             "Resource": "arn:aws:execute-api:{REGION}:{AWS_ACCOUNT}:{YOUR_API_ID}/{YOUR_API_STAGE}/OPTIONS/*"
	//         }
	//     ]
	// }
	// https://docs.aws.amazon.com/sdk-for-go/api/service/apigateway/#APIGateway.CreateApiKey
	gateway, err := gatewayClient.CreateRestApi(context.TODO(), APIInput)
	handleErrors.Check(err)

	resources, err := gatewayClient.GetResources(context.TODO(), &apigateway.GetResourcesInput{
		RestApiId: gateway.Id,
	})
	handleErrors.Check(err)

	trie := new("functions")
	trie.createResources(gatewayClient, gateway.Id, resources.Items[0].Id)

	deploymentConfig := &apigateway.CreateDeploymentInput{
		RestApiId: gateway.Id,
	}

	deployment, err := gatewayClient.CreateDeployment(context.TODO(), deploymentConfig)
	handleErrors.Check(err)

	stage, err := gatewayClient.CreateStage(context.TODO(), &apigateway.CreateStageInput{
		DeploymentId: deployment.Id,
		RestApiId:    gateway.Id,
		StageName:    aws.String("test"),
	})

	handleErrors.Check(err)

	for _, v := range resourceFunctions {
		_, err := lamClient.AddPermission(context.TODO(), &lambda.AddPermissionInput{
			Action:       aws.String("lambda:*"),
			FunctionName: &v,
			Principal:    aws.String("*"),
			StatementId:  aws.String("12315"),
		})
		handleErrors.Check(err)
	}
	stageURL = fmt.Sprintf("https://%s.execute-api.us-east-1.amazonaws.com/%s/", *gateway.Id, *stage.StageName)
	return
}
