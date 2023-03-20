import { AfterViewChecked, Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { MessageItem } from './api.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css']
})
export class AppComponent implements OnInit, AfterViewChecked {
  loading = false;
  title = 'llama-go';
  messages: MessageItem[] = [];
  robotMsg: MessageItem|null = null;
  maxTokens = 1024;

  prompt: string = '';

  wsUrl: string = '';
  wsock: WebSocket|null = null;
  @ViewChild('messageContainer') container: ElementRef | undefined;

  constructor() {
    let wsProto = 'ws';
    if (window.location.protocol === 'https') {
      wsProto = 'wss';
    }
    this.wsUrl = wsProto + '://' + window.location.host + '/api/ws/completion';
  }

  private scrollToBottom() {
    try {
        if (this.container !== undefined) {
            this.container.nativeElement.scrollTop = this.container.nativeElement.scrollHeight;
        }
    } catch (err) {
        console.log(err);
    }
  }

  private finishLastMsg() {
    let msgItem = this.robotMsg;
    if (msgItem !== null) {
      if (msgItem.loading) {
        msgItem.loading = false;
      }
      if (msgItem.text === '') {
        this.messages.pop();
      }
    }
    this.robotMsg = null;
  }

  private reload() {
    this.loading = true;
    if (this.wsock !== null) {
      this.wsock.close();
    }
    const wsock = new WebSocket(this.wsUrl);
    wsock.addEventListener('open', (e) => {
      console.log('WS Open', e);
      this.loading = false;
    });
    wsock.addEventListener('close', (e) => {
      console.log('WS Close', e);
      this.wsock = null;
      this.loading = false;
      this.finishLastMsg();
      this.reload();
    });
    wsock.addEventListener('error', (e) => {
      console.log('WS Error', e);
      this.finishLastMsg();
      this.reload();
    });
    wsock.addEventListener('message', (ev) => {
      let data = ev.data;
      try {
        let msg = JSON.parse(data);
        if (msg.finish) {
          // Finish just stop loading and return.
          this.loading = false;
          this.robotMsg = null;
          console.log(msg.reason, msg.error);
          return
        } else {
          let msgItem = this.robotMsg;
          if (msgItem !== null) {
            if (msgItem.loading) {
              msgItem.text = msg.text;
              msgItem.loading = false;
            } else {
              msgItem.text += msg.text;
            }
          }
        }
      } catch(e) {
        console.log(e);
        this.loading = false;
        this.finishLastMsg();
        this.reload();
      }
    });
    this.wsock = wsock;
  }

  ngOnInit() {
    this.reload();
    this.scrollToBottom();
  }

  ngAfterViewChecked() {
    this.scrollToBottom();
  }

  getMsgType(msg: MessageItem) {
    if (msg.role === 'user') {
      return 'info';
    }
    return 'success';
  }

  getMsgIcon(msg: MessageItem) {
    if (msg.role === 'user') {
      return 'user';
    }
    return 'robot';
  }

  private appendMessage(msg: string, mtp: string) {
    this.messages.push({
      role: mtp,
      text: msg,
      loading: false,
    })
  }

  async processRequest(prompt: string) {
    if (this.wsock === null) {
      this.reload();
    }
    const params = {
      prompt: prompt + '\n',
      tokens: this.maxTokens,
      stream: true,
    };
    let msgItem: MessageItem ={
      text: '',
      role: 'robot',
      loading: true,
    };
    this.messages.push(msgItem);
    this.loading = true;
    this.robotMsg = msgItem;
    try {
      this.wsock!.send(JSON.stringify(params));
    } catch(e) {
      console.log(e);
      this.finishLastMsg();
      if (this.wsock !== null) {
        this.wsock.close();
      }
      this.wsock = null
      this.loading = false;
    }
  }

  async send() {
    if (this.prompt === '') {
      return
    }
    const prompt = this.prompt;
    this.prompt = '';
    this.appendMessage(prompt, 'user');
    this.processRequest(prompt);
  }
}
