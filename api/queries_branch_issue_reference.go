package api

import (
	"fmt"

	"github.com/cli/cli/v2/internal/ghrepo"
)

type BranchIssueReference struct {
	ID         string
	BranchName string
}

func nameParam(params map[string]interface{}) string {
	if params["name"] != "" {
		return "name: $name,"
	}
	return ""
}

func nameArg(params map[string]interface{}) string {
	if params["name"] != "" {
		return "$name: String, "
	}

	return ""
}

func CreateBranchIssueReference(client *Client, repo *Repository, params map[string]interface{}) (*BranchIssueReference, error) {
	query := fmt.Sprintf(`
		mutation CreateLinkedBranch($issueId: ID!, $oid: GitObjectID!, %[1]s$repositoryId: ID) {
			createLinkedBranch(input: {
			  issueId: $issueId,
			  %[2]s
			  oid: $oid,
			  repositoryId: $repositoryId
			}) {
				linkedBranch {
					id
					ref {
						name
					}
				}
			}
		}`, nameArg(params), nameParam(params))

	inputParams := map[string]interface{}{
		"repositoryId": repo.ID,
	}
	for key, val := range params {
		switch key {
		case "issueId", "name", "oid":
			inputParams[key] = val
		}
	}

	result := struct {
		CreateLinkedBranch struct {
			LinkedBranch struct {
				ID  string
				Ref struct {
					Name string
				}
			}
		}
	}{}

	err := client.GraphQL(repo.RepoHost(), query, inputParams, &result)
	if err != nil {
		return nil, err
	}

	ref := BranchIssueReference{
		ID:         result.CreateLinkedBranch.LinkedBranch.ID,
		BranchName: result.CreateLinkedBranch.LinkedBranch.Ref.Name,
	}
	return &ref, nil

}

func ListLinkedBranches(client *Client, repo ghrepo.Interface, issueNumber int) ([]string, error) {
	query := `
	query BranchIssueReferenceListLinkedBranches($repositoryName: String!, $repositoryOwner: String!, $issueNumber: Int!) {
		repository(name: $repositoryName, owner: $repositoryOwner) {
			issue(number: $issueNumber) {
				linkedBranches(first: 30) {
					edges {
						node {
							ref {
								name
							}
						}
					}
				}
			}
		}
	}
	`
	variables := map[string]interface{}{
		"repositoryName":  repo.RepoName(),
		"repositoryOwner": repo.RepoOwner(),
		"issueNumber":     issueNumber,
	}

	result := struct {
		Repository struct {
			Issue struct {
				LinkedBranches struct {
					Edges []struct {
						Node struct {
							Ref struct {
								Name string
							}
						}
					}
				}
			}
		}
	}{}

	err := client.GraphQL(repo.RepoHost(), query, variables, &result)
	var branchNames []string
	if err != nil {
		return branchNames, err
	}

	for _, edge := range result.Repository.Issue.LinkedBranches.Edges {
		branchNames = append(branchNames, edge.Node.Ref.Name)
	}

	return branchNames, nil

}

// introspects the schema to see if we expose the LinkedBranch type
func CheckLinkedBranchFeature(client *Client, host string) (err error) {
	var featureDetection struct {
		Name struct {
			Fields []struct {
				Name string
			}
		} `graphql:"LinkedBranch: __type(name: \"LinkedBranch\")"`
	}

	err = client.Query(host, "LinkedBranch_fields", &featureDetection, nil)

	if err != nil {
		return err
	}

	if len(featureDetection.Name.Fields) == 0 {
		return fmt.Errorf("the `gh issue develop` command is not currently available")
	}
	return nil
}

// This fetches the oids for the repo's default branch (`main`, etc) and the name the user might have provided in one shot.
func FindBaseOid(client *Client, repo *Repository, ref string) (string, string, error) {
	query := `
	query BranchIssueReferenceFindBaseOid($repositoryName: String!, $repositoryOwner: String!, $ref: String!) {
		repository(name: $repositoryName, owner: $repositoryOwner) {
			defaultBranchRef {
				target {
					oid
				}
			}
			ref(qualifiedName: $ref) {
				target {
					oid
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"repositoryName":  repo.Name,
		"repositoryOwner": repo.RepoOwner(),
		"ref":             ref,
	}

	result := struct {
		Repository struct {
			DefaultBranchRef struct {
				Target struct {
					Oid string
				}
			}
			Ref struct {
				Target struct {
					Oid string
				}
			}
		}
	}{}

	err := client.GraphQL(repo.RepoHost(), query, variables, &result)
	if err != nil {
		return "", "", err
	}
	return result.Repository.Ref.Target.Oid, result.Repository.DefaultBranchRef.Target.Oid, nil
}
