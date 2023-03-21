import { AfterViewChecked, Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { MessageItem, PromptRequest } from './api.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css']
})
export class AppComponent implements OnInit, AfterViewChecked {
  loading = false;
  settingsModal = false;
  title = 'llama-go';
  messages: MessageItem[] = [];
  robotMsg: MessageItem|null = null;
  // Params Defaults
  maxTokens: number|null = 512;
  topK: number|null = 40;
  topP: number|null = 0.95;
  temp: number|null = 0.9;
  repeatPenalty: number|null = 1.5;
  repeatLastN: number|null = 48;
  numHistoryPrompts = 3;
  maxHistoryPromptSize = 128;

  prompt: string = '';

  wsUrl: string = '';
  wsock: WebSocket|null = null;
  @ViewChild('messageContainer') container: ElementRef | undefined;
  @ViewChild('promptInput') promptInput: ElementRef | undefined;

  private onConnecting = false;
  private onConnectMsg: string = '';

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

  private focusInput() {
    try {
      if (this.promptInput !== undefined) {
        this.promptInput.nativeElement.focus();
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

  private closeWs() {
    if (this.wsock !== null) {
      this.wsock.close();
    }
    this.wsock = null;
  }

  private reload() {
    this.loading = true;
    if (this.wsock !== null) {
      this.wsock.close();
    }
    const wsock = new WebSocket(this.wsUrl);
    this.onConnecting = true;
    wsock.addEventListener('open', (e) => {
      this.onConnecting = false;
      console.log('WS Open', e);
      if (this.onConnectMsg !== '') {
        this.wsock!.send(this.onConnectMsg);
      } else {
        this.loading = false;
      }
      this.onConnectMsg = '';
    });
    wsock.addEventListener('close', (e) => {
      console.log('WS Close', e);
      this.wsock = null;
      this.loading = false;
      this.finishLastMsg();
    });
    wsock.addEventListener('error', (e) => {
      console.log('WS Error', e);
      this.finishLastMsg();
      this.closeWs();
    });
    wsock.addEventListener('message', (ev) => {
      let data = ev.data;
      try {
        let msg = JSON.parse(data);
        if (msg.finish) {
          // Finish just stop loading and return.
          this.loading = false;
          this.finishLastMsg();
          this.robotMsg = null;
          console.log(msg.reason, msg.error);
          setTimeout(() => {
            this.focusInput();
          }, 400);
          return
        } else {
          let msgItem = this.robotMsg;
          if (msgItem !== null) {
            if (msgItem.loading) {
              msgItem.text = msg.text.replaceAll('\n', '<br/>');
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
        this.closeWs();
      }
    });
    this.wsock = wsock;
  }

  ngOnInit() {
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

  private min(a: any, b: any): any {
    if (a < b) {
      return a;
    }
    return b;
  }

  private createParameter(): PromptRequest {
    var prompts: string[] = [];
    for (var i = 0; i < this.min(this.numHistoryPrompts, this.messages.length); i++) {
      let idx = this.messages.length - 1 - i;
      var text = this.messages[idx].text;
      if (text.length > this.maxHistoryPromptSize) {
        text = text.slice(0, this.maxHistoryPromptSize);
      }
      prompts.push(text);
    }
    prompts.reverse()
    return {
      prompt: prompts.join('\n') + '\n',
      stream: true,
      tokens: (typeof this.maxTokens === 'string') ? null : this.maxTokens,
      top_k: (typeof this.topK === 'string') ? null : this.topK,
      top_p: (typeof this.topP === 'string') ? null : this.topP,
      temp: (typeof this.temp === 'string') ? null : this.temp,
      repeat_penalty: (typeof this.repeatPenalty === 'string') ? null : this.repeatPenalty,
      repeat_lastn: (typeof this.repeatLastN === 'string') ? null : this.repeatLastN,
    }
  }

  processRequest(prompt: string) {
    if (this.wsock === null) {
      this.reload();
    }
    const params = this.createParameter();
    console.log('REQ PARAMETER:', params)
    let msgItem: MessageItem ={
      text: '',
      role: 'robot',
      loading: true,
    };
    this.messages.push(msgItem);
    this.loading = true;
    this.robotMsg = msgItem;
    try {
      if (this.onConnecting) {
        this.onConnectMsg = JSON.stringify(params);
      } else {
        this.wsock!.send(JSON.stringify(params));
      }
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

  send() {
    if (this.prompt === '') {
      return
    }
    const prompt = this.prompt;
    this.prompt = '';
    this.appendMessage(prompt, 'user');
    this.processRequest(prompt);
  }

  showSettingsModal() {
    this.settingsModal = true;
  }

  handleOk() {
    this.settingsModal = false;
  }
}