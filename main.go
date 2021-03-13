package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/graph"
	"github.com/joho/godotenv"
)

// init is invoked before main()
func init() {
    // loads values from .env into the system
    if err := godotenv.Load(); err != nil {
        log.Print("No .env file found")
    }
}

func main() {

	orgs := os.Getenv("ORGS")

	//create array from orgs passed in
	orgsArr := strings.Split(orgs, ",")

	//create csv file
	f, err := os.Create("data.csv")

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	// var csvRows []string
	_, err1 := f.WriteString("Organization,Project Name,Permission Group,Depth,Breadcrumb,AAD Group,User Display Name,User LAN,User Email\n")
	if err1 != nil {
		log.Fatal(err)
	}

	for _,org := range orgsArr {

		log.Println("ORGS: " + org)
		organizationURL := "https://dev.azure.com/" + org
		personalAccessToken := os.Getenv("PAT")

		// Create a connection to your organization
		connection := azuredevops.NewPatConnection(organizationURL, personalAccessToken)

		ctx := context.Background()

		graphClient, err := graph.NewClient(ctx, connection)

		if err != nil {
			panic("Problem getting started with graph client")
		}

		recurseForContinuationToken(ctx, graphClient, &org, f, graph.ListGroupsArgs{})

	}
}

func recurseForContinuationToken(ctx context.Context, graphClient graph.Client, org *string, f *os.File, graphListGroupArgs graph.ListGroupsArgs) {

	groups, err := graphClient.ListGroups(ctx, graphListGroupArgs)

	if err == nil {

		for _, graphGroupRef := range *groups.GraphGroups {

			
			if strings.Contains(*graphGroupRef.Domain, "/TeamProject/") {
			
				groupDisplayName := *graphGroupRef.DisplayName
				// log.Println(groupDisplayName)
				// log.Println(*graphGroupRef.Url)
				// log.Println(*graphGroupRef.Description)
				// log.Println(*graphGroupRef.Descriptor)
				// log.Println(*graphGroupRef.Domain)
				// log.Println(*graphGroupRef.LegacyDescriptor)
				// log.Println(*graphGroupRef.MailAddress)
				// log.Println(*graphGroupRef.Origin)
				// log.Println(*graphGroupRef.OriginId)

				principalNameSplit := strings.Split(*graphGroupRef.PrincipalName, "\\")
				projectName := strings.TrimRight(strings.TrimLeft(principalNameSplit[0], "["), "]")
				log.Println(*graphGroupRef.PrincipalName)

				// fmt.Println("")

				recurseGroup(ctx, graphClient, org, graphGroupRef.Descriptor, 0, "", &projectName, &groupDisplayName, f)

				// fmt.Println("")
				// fmt.Println("--------------------------------------------------------------")
				// fmt.Println("--------------------------------------------------------------")

			}

		}

		if((*groups.ContinuationToken)[0] != ""){

			fmt.Println("")
			log.Println("---CONTINUATION TOKEN----")
			fmt.Println("")
			// log.Println(len(*groups.ContinuationToken))
			// log.Println(*groups.ContinuationToken)
			// log.Println((*groups.ContinuationToken)[0])
			// log.Println(&(*groups.ContinuationToken)[0])
			
			recurseForContinuationToken(ctx, graphClient, org, f, graph.ListGroupsArgs{ ContinuationToken: &(*groups.ContinuationToken)[0]})

		}
	}	

}

func recurseGroup(ctx context.Context, graphClient graph.Client, organization *string, groupDescriptor *string, depth int, breadCrumb string, projectName *string, groupName *string, f *os.File){

	vssGpMembers, err := graphClient.ListMemberships(ctx, graph.ListMembershipsArgs{
		SubjectDescriptor: groupDescriptor,
		Direction: &graph.GraphTraversalDirectionValues.Down,
	})

	if err == nil {

		for _,vssGpMemberRef := range *vssGpMembers {
			if strings.HasPrefix(*vssGpMemberRef.MemberDescriptor, "vssgp.") {

				//get the group name
				vssGpGroup, err := graphClient.GetGroup(ctx, graph.GetGroupArgs{GroupDescriptor: vssGpMemberRef.MemberDescriptor})

				if err != nil {
					panic("Get vssGpGroup error")
				}

				if breadCrumb == "" {
					breadCrumb = *vssGpGroup.DisplayName
				} else {
					breadCrumb = breadCrumb + " >> " + *vssGpGroup.DisplayName
				}
				depth++
				recurseGroup(ctx, graphClient, organization, vssGpMemberRef.MemberDescriptor, depth, breadCrumb, projectName, groupName, f)
			} else {

				if strings.HasPrefix(*vssGpMemberRef.MemberDescriptor, "aad.") {
					gUser, err := graphClient.GetUser(ctx, graph.GetUserArgs{UserDescriptor: vssGpMemberRef.MemberDescriptor})
					if err == nil {

						directoryAlias := func() string { if gUser.DirectoryAlias == nil { return "" } else { return *gUser.DirectoryAlias } }()
						email := func() string { if gUser.MailAddress == nil { return "" } else { return *gUser.MailAddress } }()
						userDisplayName := func() string { if gUser.DisplayName == nil { return "" } else { return *gUser.DisplayName } }()

						// fmt.Println(*graphMemberRef.MemberDescriptor, *gUser.DisplayName, *gUser.DirectoryAlias, *gUser.MailAddress)
						// fmt.Println(userDisplayName, directoryAlias, email)

						row := []string{*organization, *projectName, *groupName, strconv.Itoa(depth), breadCrumb, "", userDisplayName, directoryAlias, email}

						_, err := f.WriteString(strings.Join(row, ",") + "\n")
						if err != nil {
							log.Fatal(err)
						}

					}
				} else {
					if strings.HasPrefix(*vssGpMemberRef.MemberDescriptor, "aadgp.") {
						
						aadMembers, err1 := graphClient.ListMemberships(ctx, graph.ListMembershipsArgs{
							SubjectDescriptor: vssGpMemberRef.MemberDescriptor,
							Direction: &graph.GraphTraversalDirectionValues.Down,
						})

						gGroup, err2 := graphClient.GetGroup(ctx, graph.GetGroupArgs{GroupDescriptor: vssGpMemberRef.MemberDescriptor})
						if err2 != nil {
							// fmt.Println("**Error:", err2)
							panic("Get aadgp GetGroup error")
						}

						if err1 == nil {
							for _, aadMemberRef := range *aadMembers {

								if strings.HasPrefix(*aadMemberRef.MemberDescriptor, "aad.") {
									gAadGpUser, err := graphClient.GetUser(ctx, graph.GetUserArgs{UserDescriptor: aadMemberRef.MemberDescriptor})
									if err == nil {

										directoryAlias := func() string { if gAadGpUser.DirectoryAlias == nil { return "" } else { return *gAadGpUser.DirectoryAlias } }()
										email := func() string { if gAadGpUser.MailAddress == nil { return "" } else { return *gAadGpUser.MailAddress } }()
										userDisplayName := func() string { if gAadGpUser.DisplayName == nil { return "" } else { return *gAadGpUser.DisplayName } }()
										// fmt.Println(*graphMemberRef.MemberDescriptor, *gAadGpUser.DisplayName, *gAadGpUser.DirectoryAlias, *gAadGpUser.MailAddress)
										// fmt.Println(*gGroup.DisplayName, userDisplayName, directoryAlias, email)

										row := []string{*organization, *projectName, *groupName, strconv.Itoa(depth), breadCrumb, *gGroup.DisplayName, userDisplayName, directoryAlias, email}

										_, err := f.WriteString(strings.Join(row, ",") + "\n")
										if err != nil {
											log.Fatal(err)
										}
										
									} else {
										// fmt.Println("**Error:", err)
										panic("Get aad user in aadgp GetUser error")
									}
								} else {
									fmt.Println("****aadgp", *gGroup.DisplayName, *aadMemberRef.MemberDescriptor)
								}
								
							}
							
						}
					}
					
				}

			}
			
		}

	}else {
		panic("!! ERROR: vssGpMembers ListMemberships error")
	}


}