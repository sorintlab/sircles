import React, { PropTypes } from 'react'
import { graphql, compose } from 'react-apollo'
import gql from 'graphql-tag'
import { Link } from 'react-router-dom'
import { Popup } from 'semantic-ui-react'
import * as d3 from 'd3'

import Util from '../modules/Util'
import { withError } from '../modules/Error'

import OrgChartDetail from './OrgChartDetail'

const diameter = 480
const radius = diameter / 2
const margin = 20

const viewBox = {
  x1: -225,
  y1: -margin / 2,
  x2: 1000,
  y2: diameter + margin
}

class OrgChart extends React.Component {

  componentWillMount () {
    this.resetComponent()
  }

  resetComponent = () => this.setState({view: [radius, radius, diameter], transitionView: [radius, radius, diameter], node: null, nodes: null})

  componentWillReceiveProps (nextProps) {
    if (nextProps.rolesQuery.error) {
      this.props.appError.setError(true)
      return
    }

    if (nextProps.rolesQuery.loading) {
      return
    }

    // since we want to augment the received rootRoles in nodes() we take a deep
    // clone of props.rootRole (changing our props looks like an antipattern)
    let { node, nodes } = this.state

    if (!nodes || this.props.rolesQuery.rootRole !== nextProps.rolesQuery.rootRole) {
      const rootRole = JSON.parse(JSON.stringify(nextProps.rolesQuery.rootRole))
      nodes = this.nodes(rootRole)
      node = nodes[0]
    }

    if (nextProps.match.params.node) {
      if (nextProps.match.params.node !== node.data.uid) {
        for (let i = 0; i < nodes.length; i++) {
          if (nodes[i].data.uid === nextProps.match.params.node) {
            node = nodes[i]
            break
          }
        }
      }
    }

    // if new location and no node param in url reset the component and refetch the rolesQuery
    if (this.props.location !== nextProps.location) {
      if (!nextProps.match.params.node) {
        node = nodes[0]
        nextProps.rolesQuery.refetch()
      }
    }

    if (node !== this.state.node) this.zoom(node)
    this.setState({ node: node, nodes: nodes })
  }

  nodes (rootRole) {
    const pack = d3.pack()
    .size([diameter - margin, diameter - margin])
    .padding(3)

    const addType = (d) => {
      if (!d.type) {
        d.roleType === 'circle' ? d.type = 'circle' : d.type = 'role'
      }
      return d
    }

    rootRole = addType(rootRole)

    let root = d3.hierarchy(rootRole, function children (d) {
      if (d.type === 'circle') {
        let roles = []
        d.roles.forEach(function (r) {
          roles.push(addType(r))
        })
        return [...roles, {uid: d.uid, name: d.name, type: 'title', depth: d.depth}]
      }
    })
    .sum(function (d) {
      if (d.type === 'role') return 1 / (d.depth + 1)
      // if (d.type === 'title') return d.value / (d.depth + 1)
      return
    })

    root.each((node) => {
      if (node.data.type !== 'title') return
      node.data.value = node.parent.value * 40 / 100
    })

    root.sum(function (d) {
      if (d.type === 'role') return 1 / (d.depth + 1)
      if (d.type === 'title') return d.value / (d.depth + 1)
      return
    })
    .sort(function (a, b) {
      // Group togheter roles and put them on top of the list
      if (a.data.type === 'role') return -1
      if (b.data.type === 'role') return 1
      // then circles
      if (a.data.type === 'circle') return -1
      if (b.data.type === 'circle') return 1
    })

    return pack(root).descendants()
  }

  zoom (node) {
    const transitionView = this.state.transitionView
    const newView = [node.x, node.y, node.r * 2]
    this.setState({node: node, view: newView})

    var transition = d3.transition()
        .duration(750)
        .tween('zoom', (d) => {
          var i = d3.interpolateZoom(transitionView, newView)
          return (t) => {
            this.setState({transitionView: i(t)})
          }
        })

    transition.on('start', () => { this.setState({transitioning: true}) })
    transition.on('end', () => { this.setState({transitioning: false}) })
    transition.on('interrupt', () => { this.setState({transitioning: false}) })
  }

  isOutsideViewBox (x, y, r) {
    return (x + r < -viewBox.x2 / 2) ||
      (x - r > viewBox.x2 / 2) ||
      (y + r < -viewBox.y2) ||
      (y - r > viewBox.y2)
  }

  circles (ix, iy, ir) {
    const timeLine = this.props.match.params.timeLine
    const { nodes, transitioning } = this.state
    const zoomnode = this.state.node

    const k = diameter / ir

    return nodes.map(node => {
      if (node.data.type !== 'circle' && node.data.type !== 'role') return

      const x = (node.x - ix) * k
      const y = (node.y - iy) * k
      const r = node.r * k
      const transform = 'translate(' + x + ',' + y + ')'

      if (r < 5) return
      if (this.isOutsideViewBox(x, y, r)) return

      let fill = d3.color('white')
      if (node.data.roleType === 'leadlink' ||
      node.data.roleType === 'replink' ||
      node.data.roleType === 'facilitator' ||
      node.data.roleType === 'secretary') {
        fill = d3.color('#9cd8ff')
      } else if (node.data.roleType === 'circle') {
        fill = d3.color('hsl(0, 0%, 97%)')
      } else {
        // normal role
        if (node.data.roleMembers && node.data.roleMembers.length > 0) {
          fill = d3.color('#c8e6c9')
        } else {
          fill = d3.color('#f9e3bd')
        }
      }

      fill = fill.brighter(0.02 * (node.depth - zoomnode.depth))

      const style = {
        fill: fill
      }
      const circle = <circle key={node.data.uid} className={node.data.type} r={r} transform={transform} style={style} onClick={(e) => { e.preventDefault(); this.props.history.push(Util.orgChartUrl(node.data.uid, timeLine)) }} />

      if (!transitioning && node.depth >= zoomnode.depth) {
        return <Popup key={node.data.uid} trigger={circle} position='top center' inverted content={node.data.name} />
      } else {
        return circle
      }
    })
  }

  text (ix, iy, ir) {
    const timeLine = this.props.match.params.timeLine
    const { nodes, transitioning } = this.state
    const zoomnode = this.state.node

    const k = diameter / ir

    return nodes.map(node => {
      if (node.data.type !== 'title' && node.data.type !== 'role') return

      if (node.data.depth - zoomnode.data.depth > 2) return

      const blockRatioCircle = {
        x: 2.2,
        y: 1.8
      }

      const blockRatioRole = {
        x: 1.8,
        y: 1.0
      }

      let blockRatio = blockRatioRole
      if (node.data.type === 'title') blockRatio = blockRatioCircle

      const r = node.r * k
      const x = (node.x - ix) * k - r * blockRatio.x / 2
      const y = (node.y - iy) * k - r * blockRatio.y / 2
      const transform = 'translate(' + x + ',' + y + ')'

      if (this.isOutsideViewBox(x, y, r)) return

      let fontSize = r / 30
      if (node.data.type === 'role') fontSize = r / 40

      const width = r * blockRatio.x
      const height = r * blockRatio.y

      const foStyle = {
        // ignore events on div so underlying circle is selected
        pointerEvents: 'none'
      }

      const divStyle = {
        width: width,
        height: height,
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        lineHeight: '1em',
        fontSize: fontSize + 'em',
        textAnchor: 'middle',
        // ignore events on div so underlying circle is selected
        pointerEvents: 'none'
      }

      const linkStyle = {
        pointerEvents: 'all',
        textAlign: 'center',
        wordBreak: 'break-word'
      }

      // We are using an html foreignObject because it's definetly faster than a svg text object
      return (
        <foreignObject key={node.data.uid} width={width} height={height} style={foStyle} transform={transform}>
          { !transitioning && node === zoomnode && zoomnode.data.roleType !== 'circle' &&
            <OrgChartDetail timeLine={timeLine} roleUID={zoomnode.data.uid} /> ||
            <div style={divStyle}>
              <Link key={node.data.uid} style={linkStyle} to={Util.roleUrl(node.data.uid, timeLine)}>
                {node.data.name}
              </Link>
            </div>
          }
        </foreignObject>
      )
    })
  }

  render () {
    if (!this.state.nodes) return null

    const { transitionView } = this.state
    const gTransform = 'translate(' + diameter / 2 + ',' + diameter / 2 + ')'

    const x = transitionView[0]
    const y = transitionView[1]
    const r = transitionView[2]

    return (
      <div>
        <svg className='orgchart' viewBox={`${viewBox.x1} ${viewBox.y1} ${viewBox.x2} ${viewBox.y2}`}>
          <g transform={gTransform}>
            {this.circles(x, y, r)}
            {this.text(x, y, r)}
          </g>
        </svg>
      </div>
    )
  }
}

OrgChart.propTypes = {
  rolesQuery: PropTypes.shape({
    loading: PropTypes.bool.isRequired,
    rootRole: PropTypes.object
  }).isRequired
}

// TODO(sgotti) with graphql we cannot specify a recursive query/fragment so we
// have to explicitly define the depth (now to 10). A possible solution will be to receive
// from the server a flat list of roles and their parent uid.
const RolesQuery = gql`
query rolesQuery($timeLineID: TimeLineID) {
  rootRole(timeLineID: $timeLineID) {
    ...OrgChartRoleFields
    roles {
      ...OrgChartRoleFields
      roles {
        ...OrgChartRoleFields
        roles {
          ...OrgChartRoleFields
          roles {
            ...OrgChartRoleFields
            roles {
              ...OrgChartRoleFields
              roles {
                ...OrgChartRoleFields
                roles {
                  ...OrgChartRoleFields
                  roles {
                    ...OrgChartRoleFields
                    roles {
                      ...OrgChartRoleFields
                      roles {
                        ...OrgChartRoleFields
                      }
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}

fragment OrgChartRoleFields on Role {
  uid
  name
  roleType
  depth
  roleMembers {
    focus
  }
}
`

export default compose(
graphql(RolesQuery, {
  name: 'rolesQuery',
  options: props => ({
    variables: {
      timeLineID: props.match.params.timeLine || 0
    },
    fetchPolicy: 'network-only'
  })
})
)(withError(OrgChart))
