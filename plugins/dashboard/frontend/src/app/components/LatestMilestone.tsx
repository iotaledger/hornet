import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Badge from "react-bootstrap/Badge";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class LatestMilestone extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                LSMI/LMI: {' '}
                {this.props.nodeStore.status.lsmi} {' / '}
                {this.props.nodeStore.status.lmi}
                {' '}
                {
                    this.props.nodeStore.isNodeSync ?
                        <Badge variant="success">Synced</Badge>
                        :
                        <Badge variant="warning">Not Synced</Badge>
                }
            </React.Fragment>
        );
    }
}
