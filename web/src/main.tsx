import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
// Shared selectors span pages; direct routes must not depend on prior navigation.
import "./ontology-editor.css";
import "./ontology-graph.css";
import "./business-workflows.css";
import "./product-workflows.css";
import "./http-api.css";
import "./query-publishing.css";
import "./style.css";
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
