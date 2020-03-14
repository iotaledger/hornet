import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import {ExplorerBundle} from "app/components/ExplorerBundle";
import {If} from 'tsx-control-statements/components';
import Alert from "react-bootstrap/Alert";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            hash: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerBundleQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchBundle(this.props.match.params.hash);
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.hash !== this.props.match.params.hash) {
            this.props.explorerStore.searchBundle(this.props.match.params.hash);
        }
        return null;
    }

    render() {
        let {hash} = this.props.match.params;
        let {bundles, query_loading} = this.props.explorerStore;
        let bndlEle = [];
        if (bundles) {
            bundles.forEach(bundle => {
                bndlEle.push(<ExplorerBundle key={bundle[0].hash} bundle={bundle}/>);
            });
        }
        return (
            <Container>
                <h3>Bundle {bundles !== null && <span>({bundles.length})</span>}</h3>
                <p>
                    {hash} {' '}
                </p>
                <Row className={"mb-3"}>
                    <Col>
                        {
                            query_loading && <Spinner animation="border"/>
                        }
                    </Col>
                </Row>
                <If condition={bundles !== null}>
                    {bndlEle}
                </If>
                <If condition={bundles === null}>
                    <Alert variant={"warning"}>
                        Bundle not yet available or unknown on this node!
                    </Alert>
                </If>
            </Container>
        );
    }
}
