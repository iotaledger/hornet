import * as React from 'react';
import {KeyboardEvent} from 'react';
import Container from "react-bootstrap/Container";
import {inject, observer} from "mobx-react";
import {Link} from 'react-router-dom';
import * as VisuStore from "app/stores/VisualizerStore";
import NodeStore from "app/stores/NodeStore";
import Badge from "react-bootstrap/Badge";
import FormControl from "react-bootstrap/FormControl";
import InputGroup from "react-bootstrap/InputGroup";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from "react-bootstrap/Button";
import Popover from "react-bootstrap/Popover";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";
import {toInputUppercase} from "app/misc/Utils";
import { If } from 'tsx-control-statements/components';

interface Props {
    visualizerStore?: VisuStore.VisualizerStore;
    nodeStore?: NodeStore;
}

@inject("visualizerStore")
@inject("nodeStore")
@observer
export class Visualizer extends React.Component<Props, any> {
    updateInterval: any;

    constructor(props: Readonly<Props>) {
        super(props);
        this.state = {
            tps_new: 0,
            vertices_count: 0,
            selected_approvers_count: 0,
            selected_approvees_count: 0,
            tips_count: 0,
            solid_percentage: 0.0,
            confirmed_percentage: 0.0,
            conflicting_percentage: 0.0,
            updateTicks: 0,
            topicsRegistered: false,
        };
    }

    componentDidMount(): void {
        this.props.visualizerStore.start();
        this.updateInterval = setInterval(() => this.updateTick(), 500);
        this.props.nodeStore.registerVisualizerTopics();
    }

    componentWillUnmount(): void {
        clearInterval(this.updateInterval);
        this.setState({topicsRegistered: false})
        this.props.nodeStore.unregisterVisualizerTopics();
        this.props.visualizerStore.stop();
    }

    shouldComponentUpdate(_nextProps, nextState) {
        return this.state.updateTicks !== nextState.updateTicks;
    }

    updateTick = () => {
        if (this.props.nodeStore.websocketConnected && !this.state.topicsRegistered) {
            this.props.nodeStore.registerVisualizerTopics();
            this.setState({topicsRegistered: true})
        }

        if (!this.props.nodeStore.websocketConnected && this.state.topicsRegistered) {
            this.setState({topicsRegistered: false})
        }

        let {
            vertices, selected_approvers_count, selected_approvees_count,
            tips_count, solid_count, confirmed_count, conflicting_count
        } = this.props.visualizerStore;

        let {last_tps_metric} = this.props.nodeStore;

        this.setState(state => ({
            tps_new: last_tps_metric.new,
            vertices_count: vertices.size,
            selected_approvers_count: selected_approvers_count,
            selected_approvees_count: selected_approvees_count,
            tips_count: tips_count,
            updateTicks: state.updateTicks + 1
        }));

        if (vertices.size == 0) {
            this.setState(state => ({
                solid_percentage: 0.0,
                confirmed_percentage: 0.0,
                conflicting_percentage: 0.0,
            }));
        } else {
            this.setState(state => ({
                solid_percentage: solid_count / vertices.size * 100,
                confirmed_percentage: confirmed_count / vertices.size * 100,
                conflicting_percentage: conflicting_count / vertices.size * 100,
            }));
        }
    }

    updateVerticesLimit = (e) => {
        this.props.visualizerStore.updateVerticesLimit(e.target.value);
    }

    pauseResumeVisualizer = (e) => {
        this.props.visualizerStore.pauseResume();
    }

    updateSearch = (e) => {
        this.props.visualizerStore.updateSearch(e.target.value);
    }

    searchAndHighlight = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.visualizerStore.searchAndHighlight();
    }

    render() {
        let {selected, verticesLimit, paused, search} = this.props.visualizerStore;

        return (
            <Container fluid>
                <h3>Visualizer</h3>
                <Row className={"mb-1"}>
                    <Col xs={{span: 5}}>
                        <p>
                            <Badge pill style={{background: VisuStore.colorSolid, color: "white"}}>
                                Solid
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorUnsolid, color: "white"}}>
                                Unsolid
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorConfirmed, color: "white"}}>
                                Confirmed
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorConflicting, color: "white"}}>
                                Conflicting
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorMilestone, color: "white"}}>
                                Milestone
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorTip, color: "white"}}>
                                Tip
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorUnknown, color: "white"}}>
                                Unknown
                            </Badge>
                            {' '}
                            <Badge pill style={{background: VisuStore.colorHighlighted, color: "white"}}>
                                Highlighted
                            </Badge>
                            <br/>
                            Transactions: {this.state.vertices_count}, TPS: {this.state.tps_new}, Tips: {this.state.tips_count}<br/>
                            Confirmed: {this.state.confirmed_percentage.toFixed(2)}%, Conflicting: {this.state.conflicting_percentage.toFixed(2)}%, Solid: {this.state.solid_percentage.toFixed(2)}%<br/>
                            <If condition={!!selected}>
                                Selected: {selected ?
                                <Link to={`/explorer/tx/${selected.id}`} target="_blank" rel='noopener noreferrer'>
                                    {selected.id.substr(0, 10)}
                                </Link>
                                : "-"}
                                <br/>
                                Approvers/Approvees: {selected ?
                                <span>{this.state.selected_approvers_count}/{this.state.selected_approvees_count}</span>
                                : '-/-'}
                            </If>
                        </p>
                    </Col>
                    <Col xs={{span: 3, offset: 4}}>
                        <InputGroup className="mr-1" size="sm">
                            <InputGroup.Prepend>
                                <InputGroup.Text id="vertices-limit">Transaction Limit</InputGroup.Text>
                            </InputGroup.Prepend>
                            <FormControl
                                placeholder="limit"
                                type="number" value={verticesLimit.toString()} onChange={this.updateVerticesLimit}
                                aria-label="vertices-limit"
                                aria-describedby="vertices-limit"
                            />
                        </InputGroup>
                        <InputGroup className="mr-1" size="sm">
                            <InputGroup.Prepend>
                                <InputGroup.Text id="vertices-limit">
                                    Search TxHash/Tag
                                </InputGroup.Text>
                            </InputGroup.Prepend>
                            <FormControl
                                placeholder="search"
                                type="text" value={search} onChange={this.updateSearch} onInput={toInputUppercase}
                                aria-label="vertices-search" onKeyUp={this.searchAndHighlight}
                                aria-describedby="vertices-search"
                            />
                        </InputGroup>
                        <InputGroup className="mr-1" size="sm">
                            <OverlayTrigger
                                trigger={['hover', 'focus']} placement="left" overlay={
                                <Popover id="popover-basic">
                                    <Popover.Content>
                                        Pauses/resumes rendering the graph.
                                    </Popover.Content>
                                </Popover>}
                            >
                                <Button onClick={this.pauseResumeVisualizer} size="sm" variant="outline-secondary">
                                    {paused ? "Resume Rendering" : "Pause Rendering"}
                                </Button>
                            </OverlayTrigger>
                        </InputGroup>
                    </Col>
                </Row>
                <div className={"visualizer"} style={{
                    zIndex: -1, position: "absolute",
                    top: 0, left: 0,
                    width: "100%",
                    height: "100%",
                    background: "#222222"
                }} id={"visualizer"}/>
            </Container>
        );
    }
}
