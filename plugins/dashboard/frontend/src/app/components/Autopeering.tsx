import * as React from 'react';
import NodeStore from 'app/stores/NodeStore';
import { inject, observer } from 'mobx-react';
import { Choose, When, Otherwise } from 'tsx-control-statements/components';
import Badge from 'react-bootstrap/Badge';

interface Props {
  nodeStore?: NodeStore;
}

@inject('nodeStore')
@observer
export default class Autopeering extends React.Component<Props, any> {
  render() {
    return (
      <React.Fragment>
        <Choose>
          <When condition={this.props.nodeStore.status.autopeering_id !== ''}>
            Autopeering-ID: {this.props.nodeStore.status.autopeering_id}
          </When>
          <Otherwise>
            Autopeering-ID: <Badge variant="secondary">Disabled</Badge>
          </Otherwise>
        </Choose>
      </React.Fragment>
    );
  }
}
