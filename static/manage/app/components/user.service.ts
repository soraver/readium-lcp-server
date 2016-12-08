import { Injectable }    from '@angular/core';
import { Headers, Http } from '@angular/http';
import 'rxjs/add/operator/toPromise';
import { User } from './user';

@Injectable()
export class UserService {
  private usersUrl = 'http://localhost/users';  // THIS SHOULD BE EQUAL TO THE URL of the static webserver (or just /)
  private headers = new Headers ({'Content-Type': 'application/json'});

  constructor (private http: Http) { }
  getUsers(): Promise<User[]> {
    return this.http.get(this.usersUrl)
      .toPromise()
      .then(function (response) {
        let users: User[] = [];
        for (let jsonUser of response.json()) {
          users[users.length] = {userID: jsonUser.userID, alias: jsonUser.alias, email: jsonUser.email, password: null};
        }
        return users;
      })
      .catch(this.handleError);
  }

  create(alias: string, email: string, password: string): Promise<User> {
    return this.http
      .post(this.usersUrl, JSON.stringify(
          {'alias': alias, 'email': email, 'password': password}), {headers: this.headers})
      .toPromise()
      .then(res => res.json().data)
      .catch(this.handleError);
  }

  delete(id: number): Promise<void> {
    const url = `${this.usersUrl}/${id}`;
    return this.http.delete(url, {headers: this.headers})
      .toPromise()
      .then(() => null)
      .catch(this.handleError);
  }

  private handleError(error: any): Promise<any> {
    console.error('An error occurred', error);
    return Promise.reject(error.message || error);
  }
  getUser(id: number): Promise<User> {
      return this.getUsers()
      .then(users => users.find(user => user.userID === id));
  }
  update(user: User): Promise<User> {
    const url = `${this.usersUrl}/${user.userID}`;
    return this.http
      .put(url, JSON.stringify(user), {headers: this.headers})
      .toPromise()
      .then(() => user)
      .catch(this.handleError);
  }

}
