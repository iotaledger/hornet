import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Card from "react-bootstrap/Card";
import ExplorerStore from "app/stores/ExplorerStore";
import Table from "react-bootstrap/Table";
import FormCheck from "react-bootstrap/FormCheck";
import * as style from '../../assets/main.css';

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerLiveFeed extends React.Component<Props, any> {
    updateInterval: any;

    constructor(props: Readonly<Props>) {
        super(props);
        this.state = {
            topicsRegistered: false,
        };
    }

    componentDidMount(): void {
        this.updateInterval = setInterval(() => this.updateTick(), 500);
        this.props.nodeStore.registerExplorerTopics(this.props.explorerStore.valueOnly);
    }

    componentWillUnmount(): void {
        clearInterval(this.updateInterval);
        this.setState({topicsRegistered: false})
        this.props.nodeStore.unregisterExplorerTopics();
    }

    updateTick = () => {
        if (this.props.nodeStore.websocketConnected && !this.state.topicsRegistered) {
            this.props.nodeStore.registerExplorerTopics(this.props.explorerStore.valueOnly);
            this.setState({topicsRegistered: true})
        }

        if (!this.props.nodeStore.websocketConnected && this.state.topicsRegistered) {
            this.setState({topicsRegistered: false})
        }
    }

    render() {
        let {mssLiveFeed, txsLiveFeed} = this.props.explorerStore;
        return (
            <Row className={"mb-3"}>
                <Col>
                    <Card>
                        <Card.Body>
                            <Card.Title>Live Feed</Card.Title>
                            <Row className={"mb-3"}>
                                <Col md={6} xs={12}>
                                    <h6>Milestones</h6>
                                    <Table responsive>
                                        <thead>
                                        <tr>
                                            <td>#</td>
                                            <td>Hash</td>
                                        </tr>
                                        </thead>
                                        <tbody className={style.monospace}>
                                        {mssLiveFeed}
                                        </tbody>
                                    </Table>
                                </Col>
                                <Col md={6} xs={12} className='mt-3 mt-md-0'>
                                    <h6>
                                        <div className="d-flex justify-content-between align-items-center">
                                            Transactions
                                            <FormCheck inline
                                                       id={"check-value-only"}
                                                       label={"Only show value Tx"}
                                                       type={"switch"}
                                                       checked={this.props.explorerStore.valueOnly}
                                                       onChange={this.props.explorerStore.toggleValueOnly}
                                            />
                                        </div>
                                    </h6>
                                    <Table responsive>
                                        <thead>
                                        <tr>
                                            <td>Hash</td>
                                            <td>Value</td>
                                        </tr>
                                        </thead>
                                        <tbody className={style.monospace}>
                                        {txsLiveFeed}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                    </Card>
                </Col>
            </Row>
        );
    }
}
