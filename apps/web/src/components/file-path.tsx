import React, { Fragment } from "react";

export function FilePath({ path }: { path: string }) {
  return path.split("/").map((segment, index) => (
    <Fragment key={index}>
      {index > 0 ? <>/<wbr /></> : null}
      {segment}
    </Fragment>
  ));
}
