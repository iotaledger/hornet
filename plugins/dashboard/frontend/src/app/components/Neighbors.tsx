import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {Neighbor} from "app/components/Neighbor";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Neighbors extends React.Component<Props, any> {
    updateInterval: any;

    constructor(props: Readonly<Props>) {
        super(props);
        this.state = {
            topicsRegistered: false,
        };
    }

    componentDidMount(): void {
        this.updateInterval = setInterval(() => this.updateTick(), 500);
        this.props.nodeStore.registerNeighborTopics();
    }

    componentWillUnmount(): void {
        clearInterval(this.updateInterval);
        this.props.nodeStore.unregisterNeighborTopics();
    }

    updateTick = () => {
        if (this.props.nodeStore.websocketConnected && !this.state.topicsRegistered) {
            this.props.nodeStore.registerNeighborTopics();
            this.setState({topicsRegistered: true})
        }

        if (!this.props.nodeStore.websocketConnected && this.state.topicsRegistered) {
            this.setState({topicsRegistered: false})
        }
    }

    render() {
        let neighborsEle = [];
        this.props.nodeStore.neighbor_metrics.forEach((v, k) => {
            neighborsEle.push(<Neighbor key={k} identity={k}/>);
        });
        return (
            <Container fluid>
                <h3>Neighbors {neighborsEle.length > 0 && <span>({neighborsEle.length})</span>}</h3>
                <p>
                    Currently connected and disconnected neighbors known to the node.
                </p>
                {neighborsEle}
            </Container>
        );
    }
}
