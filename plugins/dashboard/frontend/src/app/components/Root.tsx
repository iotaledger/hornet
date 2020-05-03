import * as React from 'react';
import {inject, observer} from "mobx-react";
import NodeStore from "app/stores/NodeStore";
import Navbar from "react-bootstrap/Navbar";
import Nav from "react-bootstrap/Nav";
import {Dashboard} from "app/components/Dashboard";
import Badge from "react-bootstrap/Badge";
import {RouterStore} from 'mobx-react-router';
import {Explorer} from "app/components/Explorer";
import {NavExplorerSearchbar} from "app/components/NavExplorerSearchbar";
import {Redirect, Route, Switch} from 'react-router-dom';
import {LinkContainer} from 'react-router-bootstrap';
import {ExplorerTransactionQueryResult} from "app/components/ExplorerTransactionQueryResult";
import {ExplorerBundleQueryResult} from "app/components/ExplorerBundleQueryResult";
import {ExplorerAddressQueryResult} from "app/components/ExplorerAddressResult";
import {ExplorerTagQueryResult} from "app/components/ExplorerTagResult";
import {Explorer404} from "app/components/Explorer404";
import {Misc} from "app/components/Misc";
import {Neighbors} from "app/components/Neighbors";
import {Visualizer} from "app/components/Visualizer";
import {Explorer420} from "app/components/Explorer420";
import {Helmet} from 'react-helmet'

interface Props {
    history: any;
    routerStore?: RouterStore;
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@inject("routerStore")
@observer
export class Root extends React.Component<Props, any> {
    renderDevTool() {
        if (process.env.NODE_ENV !== 'production') {
            const DevTools = require('mobx-react-devtools').default;
            return <DevTools/>;
        }
    }

    componentDidMount(): void {
        this.props.nodeStore.connect();
    }

    render() {
        return (
            <div className="container">
                <Helmet defer={false}>
                    <title>{this.props.nodeStore.documentTitle}</title>
                </Helmet>
                <Navbar expand="lg" bg="light" variant="light" className={"mb-4"}>
                    <Navbar.Brand>
                        <img
                            src="/assets/favicon.svg"
                            width="40"
                            className="d-inline-block"
                            alt="Hornet"
                        />
                    </Navbar.Brand>
                    <Nav className="mr-auto">
                        <LinkContainer to="/dashboard">
                            <Nav.Link>Dashboard</Nav.Link>
                        </LinkContainer>
                        <LinkContainer to="/neighbors">
                            <Nav.Link>Neighbors</Nav.Link>
                        </LinkContainer>
                        <LinkContainer to="/explorer">
                            <Nav.Link>
                                Tangle Explorer
                            </Nav.Link>
                        </LinkContainer>
                        <LinkContainer to="/visualizer">
                            <Nav.Link>
                                Visualizer
                            </Nav.Link>
                        </LinkContainer>
                        <LinkContainer to="/debug">
                            <Nav.Link>
                                Misc
                            </Nav.Link>
                        </LinkContainer>
                    </Nav>
                    <a href="https://github.com/gohornet/hornet">
                        <svg xmlns="http://www.w3.org/2000/svg"
                             className="navbar-brand"
                             width="32"
                             height="32"
                             viewBox="0 0 16 16"
                             focusable="false">
                            <path fill="currentColor"
                                  fillRule="evenodd"
                                  d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                        </svg>
                    </a>
                    <Navbar.Collapse className="justify-content-end">
                        <NavExplorerSearchbar/>
                        <Navbar.Text>
                            {!this.props.nodeStore.websocketConnected &&
                            <Badge variant="danger">WS not connected!</Badge>
                            }
                        </Navbar.Text>
                    </Navbar.Collapse>
                </Navbar>
                <Switch>
                    <Route exact path="/dashboard" component={Dashboard}/>
                    <Route exact path="/debug" component={Misc}/>
                    <Route exact path="/neighbors" component={Neighbors}/>
                    <Route exact path="/explorer/tx/:hash" component={ExplorerTransactionQueryResult}/>
                    <Route exact path="/explorer/bundle/:hash" component={ExplorerBundleQueryResult}/>
                    <Route exact path="/explorer/addr/:hash" component={ExplorerAddressQueryResult}/>
                    <Route exact path="/explorer/tag/:hash" component={ExplorerTagQueryResult}/>
                    <Route exact path="/explorer/404/:search" component={Explorer404}/>
                    <Route exact path="/explorer/420" component={Explorer420}/>
                    <Route exact path="/explorer" component={Explorer}/>
                    <Route exact path="/visualizer" component={Visualizer}/>
                    <Redirect to="/dashboard"/>
                </Switch>
                {this.props.children}
                {this.renderDevTool()}
            </div>
        );
    }
}
