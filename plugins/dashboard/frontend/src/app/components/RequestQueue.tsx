import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class RequestQueue extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Request Queue
                Size: {this.props.nodeStore.status.request_queue_queued + this.props.nodeStore.status.request_queue_pending}
            </React.Fragment>
        );
    }
}
