export function excludedSource(name: string): boolean {
  return name.split("/").some(part => {const v=part.toLowerCase();return [".git","node_modules",".shipper",".ssh",".aws",".env","id_rsa","id_ed25519"].includes(v)||v.startsWith(".env.")&&![".env.example",".env.sample",".env.template"].includes(v)||v.endsWith(".pem")||v.endsWith(".key");});
}
export function sourceArchive(files: {name:string;file:File}[]): Blob {
  const chunks: BlobPart[]=[],encoder=new TextEncoder();let total=0;
  if (!files.length||files.length>10000) throw new Error("Choose between 1 and 10000 source files");
  for (const entry of files) {
    const path=entry.name;if(path.startsWith("/")||path.split("/").includes("..")||/[\\:\0]/.test(path)||encoder.encode(path).length>100)throw new Error("Source paths must fit the portable 100-byte tar format");
    total+=entry.file.size;if(total>64*1024*1024)throw new Error("Source is limited to 64 MiB");
    const header=new Uint8Array(512);const text=(offset:number,value:string)=>header.set(encoder.encode(value),offset);
    text(0,path);text(100,"0000644\0");text(108,"0000000\0");text(116,"0000000\0");text(124,entry.file.size.toString(8).padStart(11,"0")+"\0");text(136,"00000000000\0");text(148,"        ");text(156,"0");text(257,"ustar\0");text(263,"00");const checksum=header.reduce((a,b)=>a+b,0);text(148,checksum.toString(8).padStart(6,"0")+"\0 ");
    chunks.push(header,entry.file);const padding=(512-entry.file.size%512)%512;if(padding)chunks.push(new Uint8Array(padding));
  }
  chunks.push(new Uint8Array(1024));return new Blob(chunks,{type:"application/x-tar"});
}
