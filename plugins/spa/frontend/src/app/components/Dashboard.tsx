import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Uptime from "app/components/Uptime";
import Version from "app/components/Version";
import LatestMilestone from "app/components/LatestMilestone";
import RequestQueue from "app/components/RequestQueue";
import TPSChart from "app/components/TPSChart";
import RequestQueueChart from "app/components/RequestQueueChart";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import TipSelChart from "app/components/TipSelChart";
import ListGroup from "react-bootstrap/ListGroup";
import Card from "react-bootstrap/Card";
import MemChart from "app/components/MemChart";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Dashboard extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Dashboard</h3>
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
                                        </ListGroup>
                                    </Col>
                                    <Col>
                                        <ListGroup variant={"flush"}>
                                            <ListGroup.Item><Version/></ListGroup.Item>
                                            <ListGroup.Item><RequestQueue/></ListGroup.Item>
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
                <Row className={"mb-3"}>
                    <Col><TipSelChart/></Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><RequestQueueChart/></Col>
                </Row>
            </Container>
        );
    }
}
