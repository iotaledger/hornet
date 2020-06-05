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

interface Props {
    visualizerStore?: VisuStore.VisualizerStore;
    nodeStore?: NodeStore;
}

@inject("visualizerStore")
@inject("nodeStore")
@observer
export class Visualizer extends React.Component<Props, any> {

    componentDidMount(): void {
        this.props.visualizerStore.start();
    }

    componentWillUnmount(): void {
        this.props.visualizerStore.stop();
        this.props.nodeStore.registerHandlers();
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

    toggleBackgroundDataCollection = () => {
        if (this.props.nodeStore.collecting) {
            this.props.nodeStore.unregisterHandlers();
            return;
        }
        this.props.nodeStore.registerHandlers();
    }

    render() {
        let {
            vertices, solid_count, confirmed_count, selected,
            selected_approvers_count, selected_approvees_count,
            verticesLimit, tips_count, paused, search
        } = this.props.visualizerStore;
        let {last_tps_metric, collecting} = this.props.nodeStore;
        let solid_percentage = 0.0;
        let confirmed_percentage = 0.0;

        if (vertices.size != 0) {
            solid_percentage = solid_count / vertices.size*100
            confirmed_percentage = confirmed_count / vertices.size*100
        }

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
                            <Badge pill style={{background: VisuStore.colorMilestone, color: "white"}}>
                                Milestone
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
                            Transactions: {vertices.size}, TPS: {last_tps_metric.new}, Tips: {tips_count}<br/>
                            Confirmed/Unconfirmed: {confirmed_count}/{vertices.size - confirmed_count} ({confirmed_percentage.toFixed(2)}%)<br/>
                            Solid/Unsolid: {solid_count}/{vertices.size - solid_count} ({solid_percentage.toFixed(2)}%)<br/>
                            Selected: {selected ?
                            <Link to={`/explorer/tx/${selected.id}`} target="_blank" rel='noopener noreferrer'>
                                {selected.id.substr(0, 10)}
                            </Link>
                            : "-"}
                            <br/>
                            Approvers/Approvees: {selected ?
                            <span>{selected_approvers_count}/{selected_approvees_count}</span>
                            : '-/-'}
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
                                        Ensures that only data needed for the visualizer is collected.
                                    </Popover.Content>
                                </Popover>}
                            >
                                <Button variant="outline-secondary" onClick={this.toggleBackgroundDataCollection}
                                        size="sm">
                                    {collecting ? "Stop Background Data Collection" : "Collect Background data"}
                                </Button>
                            </OverlayTrigger>
                            <br/>
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
