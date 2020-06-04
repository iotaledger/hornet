import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {If} from 'tsx-control-statements/components';
import Badge from "react-bootstrap/Badge";
import ReactTooltip from "react-tooltip";

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
                <span data-tip={this.props.nodeStore.msDelta} data-for="ms_delta">
                    {this.props.nodeStore.status.lsmi} {' / '}
                    {this.props.nodeStore.status.lmi}
                </span>
                <If condition={!this.props.nodeStore.isNodeSync}>
                    <ReactTooltip id="ms_delta" place="bottom" effect="solid"/>
                </If>
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
