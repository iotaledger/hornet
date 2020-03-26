import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Uptime from "app/components/Uptime";
import Autopeering from "app/components/Autopeering";
import NeighborsCount from "app/components/NeighborsCount"
import Version from "app/components/Version";
import LatestMilestone from "app/components/LatestMilestone";
import PruningIndex from "app/components/PruningIndex";
import SnapshotIndex from "app/components/SnapshotIndex";
import RequestQueue from "app/components/RequestQueue";
import TPSChart from "app/components/TPSChart";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ListGroup from "react-bootstrap/ListGroup";
import Card from "react-bootstrap/Card";
import MemChart from "app/components/MemChart";
import {If} from 'tsx-control-statements/components';

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Dashboard extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Dashboard <If condition={this.props.nodeStore.status.node_alias !== ""}>({this.props.nodeStore.status.node_alias})</If></h3>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Status</Card.Title>
                                <Row>
                                    <Col>
                                        <ListGroup variant={"flush"}>
                                            <ListGroup.Item><Uptime/></ListGroup.Item>
                                            <ListGroup.Item><LatestMilestone/></ListGroup.Item>
                                            <ListGroup.Item><SnapshotIndex/></ListGroup.Item>
                                            <ListGroup.Item><PruningIndex/></ListGroup.Item>
                                        </ListGroup>
                                    </Col>
                                    <Col>
                                        <ListGroup variant={"flush"}>
                                            <ListGroup.Item><Version/></ListGroup.Item>
                                            <ListGroup.Item><RequestQueue/></ListGroup.Item>
                                            <ListGroup.Item><NeighborsCount/></ListGroup.Item>
                                            <ListGroup.Item><Autopeering/></ListGroup.Item>
                                        </ListGroup>
                                    </Col>
                                </Row>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><TPSChart/></Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><MemChart/></Col>
                </Row>
            </Container>
        );
    }
}
