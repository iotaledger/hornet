import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Button from "react-bootstrap/Button";
import {If} from 'tsx-control-statements/components';

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Version extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Version {this.props.nodeStore.status.version}
                <If condition={!this.props.nodeStore.isLatestVersion}>
                    {' '}
                    <Button href="https://github.com/gohornet/hornet/releases/latest"
                            size="sm"
                            variant="outline-info">Update available</Button>
                </If>
            </React.Fragment>
        );
    }
}
