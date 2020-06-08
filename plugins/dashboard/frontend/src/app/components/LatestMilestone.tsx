import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {OverlayTrigger, Tooltip} from "react-bootstrap";


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
                <OverlayTrigger
                    placement="bottom"
                    delay={{show: 150, hide: 150}}
                    overlay={
                        <Tooltip id={`tooltip-value`}>
                            {this.props.nodeStore.msDelta}
                        </Tooltip>
                    }
                >
                    <span>
                        {this.props.nodeStore.status.lsmi} {' / '}
                        {this.props.nodeStore.status.lmi}
                    </span>
                </OverlayTrigger>
            </React.Fragment>
        );
    }
}
