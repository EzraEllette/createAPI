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
	"github.com/google/uuid"
)

// resourceFunctions ResourceId, functionName
var resourceFunctions map[string]string
var client gatewayCreatorClient

func init() {
	resourceFunctions = map[string]string{}
	client = gatewayCreatorClient{}
}

type resourceTrieNode struct {
	Children map[string]*resourceTrieNode `json:"children"`
	Value    string                       `json:"value"`
}

type gatewayCreatorClient struct {
	Gateway *apigateway.Client `json:"client"`
	Config  aws.Config         `json:"config"`
	ApiId   *string            `json:"apiId"`
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
				Value:    string(strings.Join(pathParts, "-")),
			}
			currentNode.Children[resource] = newNode
			currentNode = currentNode.Children[resource]
		}
	}
}

func createResource(pathPart string, parentId *string) (resource *apigateway.CreateResourceOutput, err error) {
	resourceConfig := &apigateway.CreateResourceInput{
		RestApiId: client.ApiId,
		PathPart:  &pathPart,
		ParentId:  parentId,
	}
	resource, err = client.Gateway.CreateResource(context.TODO(), resourceConfig)
	return
}

func createMethodInput(resourceId *string) (err error) {
	methodInput := &apigateway.PutMethodInput{
		AuthorizationType: aws.String("NONE"),
		HttpMethod:        aws.String("ANY"),
		RestApiId:         client.ApiId,
		ResourceId:        resourceId,
		ApiKeyRequired:    false,
	}

	_, err = client.Gateway.PutMethod(context.TODO(), methodInput)
	return
}

func createMethodResponse(resourceId *string) (err error) {
	responseOptions := &apigateway.PutMethodResponseInput{
		RestApiId:      client.ApiId,
		StatusCode:     aws.String("200"),
		ResourceId:     resourceId,
		HttpMethod:     aws.String("ANY"),
		ResponseModels: map[string]string{"application/json": "Empty"},
	}

	_, err = client.Gateway.PutMethodResponse(context.TODO(), responseOptions)
	return
}

func createMethodIntegration(functionName string, resourceId *string) (err error) {
	uri := "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/" + *lambdaCreator.GetARN(functionName) + "/invocations"

	fmt.Println("adding integration for " + functionName)

	integrationConfig := &apigateway.PutIntegrationInput{
		HttpMethod:            aws.String("ANY"),
		ResourceId:            resourceId,
		RestApiId:             client.ApiId,
		Type:                  types.IntegrationTypeAws,
		IntegrationHttpMethod: aws.String("POST"),
		Uri:                   &uri,
	}

	_, err = client.Gateway.PutIntegration(context.TODO(), integrationConfig)
	return
}

func createMethodIntegrationResponse(resourceId *string) (err error) {
	integrationResponseOptions := &apigateway.PutIntegrationResponseInput{
		HttpMethod:        aws.String("ANY"),
		ResourceId:        resourceId,
		RestApiId:         client.ApiId,
		StatusCode:        aws.String("200"),
		ResponseTemplates: map[string]string{"application/json": ""},
	}

	_, err = client.Gateway.PutIntegrationResponse(context.TODO(), integrationResponseOptions)
	return
}

func (t *resourceTrieNode) createResources(apiId *string, parentId *string) {
	nodes := make(chan *resourceTrieNode, 400)

	nodes <- t

	for len(nodes) > 0 {
		currentNode := <-nodes
		for k, v := range currentNode.Children {
			resource, err := createResource(k, parentId)
			handleErrors.Check(err)

			if len(v.Children) == 0 {

				createMethodInput(resource.Id)
				handleErrors.Check(err)

				err = createMethodResponse(resource.Id)
				handleErrors.Check(err)

				err = createMethodIntegration(v.Value, resource.Id)
				handleErrors.Check(err)

				resourceFunctions[*resource.Id] = *lambdaCreator.GetARN(v.Value)

				err = createMethodIntegrationResponse(resource.Id)
				handleErrors.Check(err)
			}

			v.createResources(apiId, resource.Id)
		}
	}
}

func createTrie() *resourceTrieNode {
	return &resourceTrieNode{
		Children: map[string]*resourceTrieNode{},
		Value:    "/",
	}
}

func new(basePath string) *resourceTrieNode {
	trie := createTrie()

	eachFile.Recursive(basePath, func(filename string, file []byte) {
		if filename == basePath {
			return
		}
		trie.insert(filename[10:])
	})
	return trie
}

func createGateway() (gateway *apigateway.CreateRestApiOutput, err error) {
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

	gateway, err = client.Gateway.CreateRestApi(context.TODO(), APIInput)
	return
}

func CreateApigateway(cfg aws.Config, lamClient *lambda.Client) (stageURL string, err error) {
	gatewayClient := apigateway.NewFromConfig(cfg)
	client.Gateway = gatewayClient

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
	gateway, err := createGateway()
	handleErrors.Check(err)

	client.ApiId = gateway.Id

	resources, err := gatewayClient.GetResources(context.TODO(), &apigateway.GetResourcesInput{
		RestApiId: gateway.Id,
	})
	handleErrors.Check(err)

	trie := new("functions")
	trie.createResources(gateway.Id, resources.Items[0].Id)

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
			StatementId:  aws.String(uuid.NewString()),
		})
		handleErrors.Check(err)
	}
	stageURL = fmt.Sprintf("https://%s.execute-api.us-east-1.amazonaws.com/%s/", *gateway.Id, *stage.StageName)
	return
}
