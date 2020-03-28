import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class MilestonePendingQueue extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                ReqQ for Milestone {this.props.nodeStore.status.current_requested_ms} {' '}
                / Size: {this.props.nodeStore.status.ms_request_queue_size}
            </React.Fragment>
        );
    }
}
