import gql from 'graphql-tag'

const ViewerQuery = gql`
  query viewerQuery {
    viewer {
      member {
        uid
        isAdmin
        userName
        circles {
          role {
            uid
            depth
            name
          }
          isLeadLink
        }
        roles {
          role {
            uid
            depth
            name
            parent {
              uid
              depth
              name
            }
          }
        }
      }
    }
  }
`

export default ViewerQuery
