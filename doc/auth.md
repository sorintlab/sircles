# Authentication and user/member management

sircles always keeps the member information in its local database.

It can be configured to use different authentication methods:

* local (password are kept in the local database)
* external authentication:
 * ldap
 * openid connect

You can find a detailed and commented configuration file [here](config.example.yaml)

When using external authentication, the matching between the local member and the external authentication user is done using a special matchUID field saved in the local database. An external authenticator, after a successful authentication returns a matchUID that will be used to match a local member. If no local member is found another attempt is done matching the returned matchUID with the local member UserName (only if its matchUID is empty). If no match can be found and a member provider is defined it'll be used to retrieve the member data and the local member will be created, otherwise the authentication is rejected.

# Importing external member

When using an external authentication method, members can be manually (or programmatically using the api) created or imported using a "member provider". The member provider will use the information provided at login time by the user and/or the information provided by the authenticator (like the oidc token when using oidc auth) to retrieve the required data for creating the member in the local database. One of the required data is the matchUID that will be used in future authentications to match a local member. As a security checke, the matchUID returned by the member provider must be the same of the one returned by the authentication handler.

# changing authentication method

The basic rule, if you want to change the authentication method when the sircles database already have members, is to configure the new authentication method to provide the same matchUID of the previous one.

If you're moving from the local auth method your member will probably have an empty matchUID (if you haven't set it calling the api) so the authentication handler will try to match the matchUID with the member username. If your authentication provider cannot provide a matchUID equal to the members' usernames you should create a script to set the members matchUID to the one provided by the new auth provider.

If you're moving from an external auth method to another external auth method and you had created members manually without using a member provider then it's the same as above (empty matchUID).

If you're moving from an external auth method to another external auth method and used a member provider to automatically create members then they will have a matchUID set, if the new authentication handler can provide the same matchUID you're done, if this isn't the case you should create a script to set the members matchUID to the one provided by the new auth provider.
