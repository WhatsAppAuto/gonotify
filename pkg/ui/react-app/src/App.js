import React from 'react';
import { Switch, Route } from 'react-router-dom';

import Nav from 'Views/Nav';
import Home from 'Views/Home';
import LogReg from 'Views/LogReg';

import styles from './App.module.css';

function App() {
  return (
    <div className={styles.App}>
      <Nav />
      <Switch>
        <Route exact path="/">
          <Home />
        </Route>
        <Route exact path={['/login', '/register']}>
          <LogReg />
        </Route>
      </Switch>
    </div>
  );
}

export default App;
