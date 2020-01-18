import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Version extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Snapshot Index: {this.props.nodeStore.status.snapshot_index}
            </React.Fragment>
        );
    }
}
